"""Guard path-aware admin probing against HTML fallbacks and old root bundles."""
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch
from urllib.parse import urlsplit

spec = importlib.util.spec_from_file_location('deployment', Path(__file__).resolve().parents[1] / 'check-deployment.py')
deployment = importlib.util.module_from_spec(spec)
spec.loader.exec_module(deployment)


class AdminPathProbeTest(unittest.TestCase):
    def probe(self, bad_assets=False, old_base=False):
        edge = {'USER_DOMAIN': 'shop.test', 'ADMIN_DOMAIN': 'admin.test', 'API_DOMAIN': 'api.test',
                'ALIPAY_RETURN_URL': 'https://shop.test/pay'}
        def run(args, label):
            url = urlsplit(args[-1])
            status, mime = '200', 'text/html'
            if url.path == '/api/v1/payments/callback':
                body, mime = 'fail', 'text/plain'
            elif url.path == '/health':
                body, mime = '{"status":"ok"}', 'application/json'
            elif url.hostname == 'admin.test' or url.path == '/admin':
                body, status = '', '308'
            elif url.path.startswith('/admin/assets/'):
                body, mime = ('<!doctype html>', 'text/html') if bad_assets else ('const ok=1', 'application/javascript')
            else:
                prefix = '' if old_base else '/admin'
                body = f'<title>管理后台</title><div id="app"></div><script src="{prefix}/assets/main.js"></script>'
            return f'{body}\n{status}\n{mime}'.encode()
        with patch.object(deployment, 'run', side_effect=run):
            deployment.probe_routes(edge)

    def test_path_bundle_passes(self):
        self.probe()

    def test_rejects_asset_html_fallback(self):
        with self.assertRaises(deployment.CheckError):
            self.probe(bad_assets=True)

    def test_rejects_root_base_bundle(self):
        with self.assertRaises(deployment.CheckError):
            self.probe(old_base=True)
