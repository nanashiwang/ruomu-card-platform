#!/usr/bin/env python3
"""Build and test the production stack with isolated volumes and temporary TLS.
No production env, payment credentials or external payment request is used.
"""
import importlib.util
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import uuid

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("deployment", ROOT / "scripts/check-deployment.py")
deployment = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(deployment)


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def main():
    with tempfile.TemporaryDirectory(prefix="ruomu-routing-") as tmp:
        tmp = Path(tmp)
        os.chmod(tmp, 0o700)
        tls = tmp / "tls"
        domains = ["shop.ruomu.test", "admin.ruomu.test", "api.ruomu.test"]
        ca_bundle = tmp / "ca.pem"
        # One ephemeral SAN certificate for the three hosts, never added to system trust.
        certificate = tmp / "cert.pem"
        private_key = tmp / "key.pem"
        deployment.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "30",
                        "-subj", "/CN=ruomu-routing-test", "-addext", "subjectAltName=" + ",".join("DNS:" + d for d in domains),
                        "-keyout", str(private_key), "-out", str(certificate)], "生成临时 TLS 证书失败")
        ca_bundle.write_bytes(certificate.read_bytes())
        for domain in domains:
            directory = tls / domain
            directory.mkdir(parents=True)
            (directory / "fullchain.pem").write_bytes(certificate.read_bytes())
            (directory / "privkey.pem").write_bytes(private_key.read_bytes())
            (directory / "privkey.pem").chmod(0o600)
        http_port, https_port = free_port(), free_port()
        values = {
            "USER_DOMAIN": domains[0], "ADMIN_DOMAIN": domains[1], "API_DOMAIN": domains[2],
            "ALIPAY_NOTIFY_URL": f"https://{domains[2]}/api/v1/payments/callback",
            "ALIPAY_RETURN_URL": f"https://{domains[0]}/pay", "TLS_CERT_DIR": str(tls),
            "HTTP_BIND": "127.0.0.1", "HTTPS_BIND": "127.0.0.1", "HTTP_PORT": str(http_port), "HTTPS_PORT": str(https_port),
            "DEFAULT_ADMIN_USERNAME": "routing-test-admin", "DEFAULT_ADMIN_PASSWORD": secrets.token_hex(24) + "Aa1",
        }
        for key in ("APP_SECRET_KEY", "JWT_SECRET", "USER_JWT_SECRET", "REDIS_PASSWORD"):
            values[key] = secrets.token_hex(32)
        env_file = tmp / ".env"
        env_file.write_text("\n".join(f"{k}={v}" for k, v in values.items()) + "\n")
        env_file.chmod(0o600)
        # Compose shell overrides cannot import a developer's production values into this test.
        for name in list(os.environ):
            if name.startswith("COMPOSE_"):
                os.environ.pop(name)
        os.environ.update(values)
        os.environ["ENV_FILE"] = str(env_file)
        os.environ["COMPOSE_FILE"] = str(ROOT / "deploy/docker-compose.prod.yml")
        os.environ["COMPOSE_PROJECT_NAME"] = "ruomu-routing-" + uuid.uuid4().hex[:10]
        os.environ["DATABASE_DRIVER"] = "sqlite"
        os.environ["DATABASE_DSN"] = "/app/db/ruomu.db"
        os.environ["EMAIL_ENABLED"] = "false"
        command = deployment.compose_command()
        try:
            config = deployment.load_config(command)
            edge, directory = deployment.validate_config(config)
            deployment.check_certificates(edge, directory)
            # An unreadable key must fail before any service starts.
            key = directory / domains[0] / "privkey.pem"
            original = key.read_bytes()
            key.write_text("not a private key")
            try:
                deployment.check_certificates(edge, directory)
                raise AssertionError("无效私钥未被前置检查阻断")
            except deployment.CheckError:
                pass
            key.write_bytes(original)
            print("构建实际生产 API、用户站与管理站镜像……", flush=True)
            subprocess.run(command + ["build"], check=True, cwd=ROOT)
            subprocess.run(["bash", "scripts/deploy.sh", "--check"], check=True, cwd=ROOT)
            subprocess.run(command + ["up", "-d", "--wait", "--wait-timeout", "180"], check=True, cwd=ROOT)
            deployment.probe_routes(edge, "127.0.0.1", https_port, ca_bundle)
            for domain in domains:
                response = deployment.run([
                    "curl", "--silent", "--show-error", "--noproxy", "*", "--max-time", "10", "-D", "-", "-o", "/dev/null",
                    "--resolve", f"{domain}:{http_port}:127.0.0.1", "--data", "notify_id=probe",
                    f"http://{domain}:{http_port}/api/v1/payments/callback?probe=1",
                ], "HTTP 跳转检查失败").decode()
                assert "308 Permanent Redirect" in response, response
                assert f"Location: https://{domain}/api/v1/payments/callback?probe=1" in response, response
            # Check actual logs to distinguish handler invocation from an edge canned response.
            # release mode writes to the persistent log directory, not stdout.
            logs = deployment.run(command + ["exec", "-T", "api", "cat", "/app/logs/app.log"], "读取测试 API 日志失败").decode()
            assert logs.count("alipay_callback_payment_not_found") >= 3, "缺少三域名支付处理器日志证据"
            assert "WRONGPASS" not in logs and "NOAUTH" not in logs, "Redis 认证失败"
            health = json.loads(deployment.run(command + ["ps", "--format", "json", "api"], "读取 API 状态失败"))
            assert health["Health"] == "healthy"
            print("Docker 验证通过：真实三端构建、TLS/SNI、HTTP 308、回调处理器日志、SPA 返回页、Redis 认证。", flush=True)
        finally:
            # Only the unique test project is removed, including its disposable database.
            subprocess.run(command + ["down", "--volumes", "--remove-orphans", "--rmi", "local"], cwd=ROOT, check=True)


if __name__ == "__main__":
    main()
