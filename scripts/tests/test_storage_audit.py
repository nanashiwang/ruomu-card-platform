import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('audit', Path(__file__).resolve().parents[1] / 'storage_audit.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)

class StorageAuditTests(unittest.TestCase):
    def test_layout_is_evidence_not_deletion_permission(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder).resolve()
            (root/'00').mkdir()
            (root/'ff').mkdir()
            (root/'README').write_text('fixture')
            self.assertIn('still need review', a.cache_layout(root, 'ruomu-go-build-cache'))
            (root/'orders.db').touch()
            self.assertEqual(a.cache_layout(root, 'ruomu-go-build-cache'), 'unrecognized-layout; retain')
            (root/'link').symlink_to('/etc')
            self.assertEqual(a.cache_layout(root, 'ruomu-go-build-cache'), 'unrecognized-or-symlink')

    def test_saved_release_dependencies_are_found_without_exposing_values(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder).resolve()
            store=root/'releases'; backups=root/'backups'
            store.mkdir(); (backups/'backup').mkdir(parents=True)
            p=backups/'backup/rollback.json'
            p.write_text(json.dumps({'secret':'private-value','config':{'image':'sha256:fixture','volumes':['cache-name']}}))
            with patch.object(a,'STORE',store),patch.object(a,'BACKUPS',backups):
                self.assertEqual(a.saved_references({'cache-name'}), [str(p)])
                self.assertEqual(a.saved_references({'different-name'}), [])
            self.assertTrue(a.references({'list':['image-id']},{'image-id'}))

    def test_audit_handles_absent_candidates_and_redacts_logging_metadata(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder).resolve()
            info={'Name':'/api','Image':'id','Config':{'Labels':{'com.docker.compose.project':'ruomu-card-platform','com.docker.compose.service':'api'},'Image':'tag','Env':['SECRET=private']},'State':{'Running':True},'HostConfig':{'LogConfig':{'Type':'json-file','Config':{'tag':'secret-tag','max-size':'20m','max-file':'3'}}}}
            calls=[]
            def cmd(*args, **kwargs):
                calls.append(args)
                if args[1:]==('ps','-aq'): return 'id\n'
                if args[1:]==('inspect','id'): return json.dumps([info])
                if args[1]=='info': return str(root)
                if args[1]=='volume': return ''
                if args[1]=='image': return ''
                raise AssertionError(args)
            with patch.object(a,'ROOT',root),patch.object(a,'command',side_effect=cmd): result=a.inspect_audit()
            self.assertEqual(len(result['candidates']),4)
            self.assertTrue(all(not x['exists'] and not x['deletion_approved'] for x in result['candidates']))
            self.assertTrue(result['read_only'])
            self.assertEqual(result['deleted_objects'],0)
            text=json.dumps(result)
            self.assertNotIn('private',text); self.assertNotIn('secret-tag',text)
            self.assertFalse(any(x in {'rm','prune','stop','restart','up'} for args in calls for x in args))

    def test_symlink_path_and_malformed_backup_fail_closed(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder).resolve()
            link=root/'link'; link.symlink_to(root)
            with self.assertRaises(a.AuditError): a.resolved(link)
            store=root/'store'; store.mkdir(); (store/'current.json').write_text('not json')
            with patch.object(a,'STORE',store),patch.object(a,'BACKUPS',root/'backups'):
                with self.assertRaises(a.AuditError): a.saved_references({'name'})

if __name__ == '__main__': unittest.main()
