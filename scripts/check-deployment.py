#!/usr/bin/env python3
"""Production preflight / public HTTPS routing probe. Never print resolved secrets."""
import argparse
import ipaddress
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import uuid

ROOT = Path(__file__).resolve().parent.parent


class CheckError(Exception):
    pass


def run(args, label, **kwargs):
    result = subprocess.run(args, capture_output=True, **kwargs)
    if result.returncode:
        # Compose / openssl output can contain expanded environment or key material.
        raise CheckError(label)
    return result.stdout


def compose_command():
    env_file = Path(os.environ.get("ENV_FILE", ROOT / ".env")).resolve()
    compose_file = Path(os.environ.get("COMPOSE_FILE", ROOT / "deploy/docker-compose.prod.yml")).resolve()
    if not env_file.is_file():
        raise CheckError("缺少 ENV_FILE（默认 .env），请先复制并填写 .env.example")
    return ["docker", "compose", "--env-file", str(env_file), "-f", str(compose_file)]


def load_config(command):
    raw = run(command + ["config", "--format", "json"],
              "Compose 配置解析失败：检查 .env 必填项、文件路径及 Compose 版本")
    return json.loads(raw)


def validate_domain(value, name):
    if not isinstance(value, str) or len(value) > 253 or value != value.lower():
        raise CheckError(f"{name} 必须为小写 DNS 域名")
    labels = value.split(".")
    if len(labels) < 2 or any(not re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?", s) for s in labels):
        raise CheckError(f"{name} 必须是域名，不能包含协议、端口、路径或通配符")
    try:
        ipaddress.ip_address(value)
    except ValueError:
        pass
    else:
        raise CheckError(f"{name} 不能使用 IP 地址")
    if any(value == d or value.endswith("." + d) for d in ("example.com", "example.org", "example.net", "localhost")):
        raise CheckError(f"{name} 仍是示例或本地域名")


def validate_secret(value, name, minimum=32):
    if not value or len(value) < minimum or re.search(r"change[-_]?me|your[-_].*secret|placeholder", value, re.I):
        raise CheckError(f"{name} 未填写、长度不足或仍使用示例值")


def validate_config(config):
    services = config["services"]
    edge = services["gateway"]["environment"]
    api = services["api"]["environment"]
    domains = [edge[k] for k in ("USER_DOMAIN", "ADMIN_DOMAIN", "API_DOMAIN")]
    for name, domain in zip(("USER_DOMAIN", "ADMIN_DOMAIN", "API_DOMAIN"), domains):
        validate_domain(domain, name)
    if len(set(domains)) != 3:
        raise CheckError("用户站、管理站和 API 必须使用三个不同域名")
    expected = {
        "ALIPAY_NOTIFY_URL": f"https://{domains[2]}/api/v1/payments/callback",
        "ALIPAY_RETURN_URL": f"https://{domains[0]}/pay",
    }
    for name, url in expected.items():
        if edge.get(name) != url:
            raise CheckError(f"{name} 必须严格等于 {url}（不加尾斜杠、查询参数或 fragment）")
    secrets = []
    for key in ("APP_SECRET_KEY", "JWT_SECRET", "USER_JWT_SECRET", "REDIS_PASSWORD"):
        value = api.get(key, "")
        validate_secret(value, key)
        secrets.append(value)
    if len(set(secrets)) != len(secrets):
        raise CheckError("应用、管理员 JWT、用户 JWT 和 Redis 密钥必须分别生成")
    password = api.get("BOOTSTRAP_DEFAULT_ADMIN_PASSWORD", "")
    validate_secret(password, "DEFAULT_ADMIN_PASSWORD", 16)
    if not all(re.search(pattern, password) for pattern in (r"[a-z]", r"[A-Z]", r"[0-9]")):
        raise CheckError("DEFAULT_ADMIN_PASSWORD 至少包含大小写字母及数字")
    if not api.get("BOOTSTRAP_DEFAULT_ADMIN_USERNAME", "").strip():
        raise CheckError("DEFAULT_ADMIN_USERNAME 不可为空")
    if api.get("SERVER_MODE") != "release":
        raise CheckError("生产 API 必须使用 release 模式")
    if api.get("QUEUE_PASSWORD") != api.get("REDIS_PASSWORD") or services["redis"]["environment"].get("REDIS_PASSWORD") != api.get("REDIS_PASSWORD"):
        raise CheckError("Redis 服务、API 与队列的密码不一致")
    for name in ("api", "user", "admin", "redis"):
        if services[name].get("ports"):
            raise CheckError(f"生产 {name} 不应发布宿主机端口，请通过 gateway 访问")
    tls_mount = next((v for v in services["gateway"].get("volumes", []) if v["target"] == "/etc/nginx/tls"), None)
    if not tls_mount or not tls_mount.get("read_only") or not Path(tls_mount["source"]).is_absolute():
        raise CheckError("TLS_CERT_DIR 必须为只读挂载的绝对路径")
    return edge, Path(tls_mount["source"])


def check_certificates(edge, directory):
    for name in ("USER_DOMAIN", "ADMIN_DOMAIN", "API_DOMAIN"):
        domain = edge[name]
        cert = directory / domain / "fullchain.pem"
        key = directory / domain / "privkey.pem"
        if not cert.is_file() or not key.is_file():
            raise CheckError(f"缺少 {domain}/fullchain.pem 或 privkey.pem")
        run(["openssl", "x509", "-in", str(cert), "-noout", "-checkend", "604800"],
            f"{domain} 证书无效或将在 7 天内到期")
        # Trust the supplied chain here only to check dates/SAN; --online checks public trust.
        run(["openssl", "verify", "-partial_chain", "-trusted", str(cert), "-verify_hostname", domain,
             "-purpose", "sslserver", str(cert)], f"{domain} 证书有效期、域名或用途不匹配")
        cert_pub = run(["openssl", "x509", "-in", str(cert), "-pubkey", "-noout"], f"{domain} 无法读取证书")
        key_pub = run(["openssl", "pkey", "-in", str(key), "-pubout", "-passin", "pass:"], f"{domain} 私钥不可读或需要密码")
        if cert_pub.strip() != key_pub.strip():
            raise CheckError(f"{domain} 证书与私钥不匹配")


def probe_routes(edge, connect_ip=None, https_port=443, ca_file=None):
    def request(url, data=None):
        args = ["curl", "--silent", "--show-error", "--noproxy", "*", "--max-time", "20",
                "--write-out", "\n%{http_code}\n%{content_type}"]
        if ca_file:
            args += ["--cacert", str(ca_file)]
        if connect_ip:
            for key in ("USER_DOMAIN", "ADMIN_DOMAIN", "API_DOMAIN"):
                args += ["--connect-to", f"{edge[key]}:443:{connect_ip}:{https_port}"]
        if data:
            args += ["--data", data]
        raw = run(args + [url], "HTTPS 访问失败：检查 DNS、443、防火墙、证书信任链和网关状态").decode()
        body, status, content_type = raw.rsplit("\n", 2)
        return body, status, content_type

    # A synthetic, non-existent transaction must reach HandleAlipayCallback and return fail.
    # Never expect success without a real signature, and never change an existing payment.
    form = ("notify_id=ruomu-routing-probe&notify_type=trade_status_sync&sign=invalid-probe-signature"
            f"&out_trade_no=RUOMU-PROBE-{uuid.uuid4().hex}&trade_status=TRADE_SUCCESS&total_amount=0.01")
    for key in ("API_DOMAIN", "USER_DOMAIN", "ADMIN_DOMAIN"):
        domain = edge[key]
        body, status, content_type = request(f"https://{domain}/api/v1/payments/callback", form)
        if (body, status) != ("fail", "200") or "text/plain" not in content_type:
            raise CheckError(f"{domain} 支付回调未返回处理器预期的 200 / fail；检查自定义回调路由、鉴权、重定向及 SPA 兜底")
    body, status, _ = request(f"https://{edge['API_DOMAIN']}/health")
    if status != "200" or json.loads(body).get("status") != "ok":
        raise CheckError("API 健康检查失败")
    for url in (edge["ALIPAY_RETURN_URL"] + "?alipay_return=1&order_no=RUOMU-PROBE",
                f"https://{edge['ADMIN_DOMAIN']}/login"):
        body, status, content_type = request(url)
        if status != "200" or "text/html" not in content_type or 'id="app"' not in body:
            raise CheckError("用户 /pay 或管理站页面未到达 SPA")
    print("HTTPS 路由通过：三域名回调均到达支付宝处理器（200 / fail）；/pay、管理站和 API 健康检查通过。")
    print("该探针只证明可达，不代表商户签约、渠道参数或真实交易已验证。")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--online", action="store_true", help="通过公网 DNS 验证 HTTPS 与实际路由")
    args = parser.parse_args()
    try:
        for name in ("docker", "openssl", "curl"):
            if not shutil.which(name):
                raise CheckError(f"缺少 {name} 命令")
        command = compose_command()
        config = load_config(command)
        edge, directory = validate_config(config)
        if args.online:
            probe_routes(edge)
        else:
            run(["docker", "info"], "Docker daemon 不可用")
            check_certificates(edge, directory)
            run(command + ["run", "--rm", "--no-deps", "-T", "gateway", "nginx", "-t"],
                "Nginx 配置检查失败：检查网关镜像、挂载文件和 TLS 文件权限")
            print("前置检查通过：Compose、密钥、域名、回调路径、证书和 nginx -t。")
            print("请将以下值填入管理后台支付宝渠道（.env 不会自动同步渠道）：")
            print("notify_url=" + edge["ALIPAY_NOTIFY_URL"])
            print("return_url=" + edge["ALIPAY_RETURN_URL"])
        return 0
    except (CheckError, OSError, ValueError, KeyError) as exc:
        if isinstance(exc, CheckError):
            print("检查失败：" + str(exc), file=sys.stderr)
        else:
            print("检查失败：配置结构、证书或工具输出无效（敏感内容已隐藏）", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
