import copy
import importlib.util
from pathlib import Path
import secrets
import unittest

SPEC = importlib.util.spec_from_file_location("deployment", Path(__file__).resolve().parents[1] / "check-deployment.py")
deployment = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(deployment)


def valid_config():
    return {"services": {
        "gateway": {
            "environment": {"USER_DOMAIN": "shop.ruomu.test", "ADMIN_DOMAIN": "admin.ruomu.test", "API_DOMAIN": "api.ruomu.test",
                            "ALIPAY_NOTIFY_URL": "https://api.ruomu.test/api/v1/payments/callback",
                            "ALIPAY_RETURN_URL": "https://shop.ruomu.test/pay"},
            "volumes": [{"source": "/etc/ruomu/tls", "target": "/etc/nginx/tls", "read_only": True}],
        },
        "api": {"environment": {"APP_SECRET_KEY": secrets.token_hex(32), "JWT_SECRET": secrets.token_hex(32),
                                "USER_JWT_SECRET": secrets.token_hex(32), "REDIS_PASSWORD": "r" * 32,
                                "QUEUE_PASSWORD": "r" * 32, "SERVER_MODE": "release",
                                "BOOTSTRAP_DEFAULT_ADMIN_USERNAME": "admin",
                                "BOOTSTRAP_DEFAULT_ADMIN_PASSWORD": secrets.token_hex(16) + "Aa1"}},
        "redis": {"environment": {"REDIS_PASSWORD": "r" * 32}}, "user": {}, "admin": {},
    }}


class DeploymentValidationTest(unittest.TestCase):
    def test_valid_config(self):
        deployment.validate_config(valid_config())

    def test_reject_callback_misroutes(self):
        for key, values in {
            "ALIPAY_NOTIFY_URL": ["http://api.ruomu.test/api/v1/payments/callback", "https://api.ruomu.test/pay",
                                  "https://api.ruomu.test/api/v1/payments/callback/", "https://shop.ruomu.test/api/v1/payments/callback",
                                  "https://api.ruomu.test/api/v1/payments/callback?channel_id=1"],
            "ALIPAY_RETURN_URL": ["https://api.ruomu.test/pay", "https://shop.ruomu.test/#/pay", "https://shop.ruomu.test/pay/"],
        }.items():
            for value in values:
                with self.subTest(value=value):
                    config = valid_config()
                    config["services"]["gateway"]["environment"][key] = value
                    with self.assertRaises(deployment.CheckError):
                        deployment.validate_config(config)

    def test_reject_unsafe_domains(self):
        for value in ("shop.example.com", "127.0.0.1", "localhost", "shop.ruomu.test;return 200", "shop.ruomu.test:443", "*.ruomu.test"):
            with self.subTest(value=value), self.assertRaises(deployment.CheckError):
                deployment.validate_domain(value, "USER_DOMAIN")

    def test_reject_secrets_and_port_bypass(self):
        base = valid_config()
        mutations = [
            lambda c: c["services"]["api"]["environment"].update(JWT_SECRET="change-me-32-byte-admin-jwt-secret"),
            lambda c: c["services"]["api"]["environment"].update(USER_JWT_SECRET=c["services"]["api"]["environment"]["JWT_SECRET"]),
            lambda c: c["services"]["api"]["environment"].update(BOOTSTRAP_DEFAULT_ADMIN_PASSWORD=""),
            lambda c: c["services"]["api"]["environment"].update(QUEUE_PASSWORD="wrong"),
            lambda c: c["services"]["api"].update(ports=[{"target": 8080, "published": "8080"}]),
            lambda c: c["services"]["gateway"]["environment"].update(ADMIN_DOMAIN="shop.ruomu.test"),
        ]
        for mutate in mutations:
            config = copy.deepcopy(base)
            mutate(config)
            with self.assertRaises(deployment.CheckError):
                deployment.validate_config(config)


if __name__ == "__main__":
    unittest.main()
