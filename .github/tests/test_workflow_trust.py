"""Check the upstream intake's security boundary (requires PyYAML)."""
from pathlib import Path
import unittest
import yaml


class WorkflowTrustTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = yaml.safe_load((Path(__file__).parents[1] / 'workflows/review-upstreams.yml').read_text())

    def test_candidate_execution_has_no_write_scope_or_secret_expressions(self):
        self.assertEqual(self.workflow['permissions'], {'contents': 'read'})
        for name in ('prepare', 'test'):
            job = self.workflow['jobs'][name]
            self.assertNotIn('permissions', job)
            self.assertNotIn('secrets.', yaml.safe_dump(job))
            for step in job['steps']:
                if step.get('uses', '').startswith('actions/checkout@'):
                    self.assertFalse(step['with']['persist-credentials'])
                self.assertFalse(step.get('continue-on-error', False))

    def test_publication_is_a_separate_runner_after_all_tests(self):
        publish = self.workflow['jobs']['publish']
        self.assertEqual(set(publish['needs']), {'prepare', 'test'})
        self.assertEqual(publish['permissions'], {'contents': 'write', 'pull-requests': 'write'})
        commands = '\n'.join(step.get('run', '') for step in publish['steps'])
        self.assertNotIn('git checkout', commands)
        self.assertNotIn('git switch', commands)
        self.assertNotIn('bash .github/', commands)
        self.assertNotIn('go test', commands)
        self.assertIn('git bundle verify', commands)
        self.assertIn('force-with-lease=', commands)

    def test_nested_dependency_fixtures_are_explicitly_executed(self):
        verification = yaml.safe_load((Path(__file__).parents[1] / 'workflows/verify-proton-integration.yml').read_text())
        jobs = [self.workflow['jobs']['test'], verification['jobs']['proton-and-bisync']]
        for job in jobs:
            commands = '\n'.join(step.get('run', '') for step in job['steps'])
            self.assertIn('go test -race github.com/rclone/go-proton-api github.com/rclone/Proton-API-Bridge', commands)
            self.assertIn('MixedLegacyModern', commands)
            self.assertIn('DraftReplacement', commands)

    def test_actions_are_immutable_and_artifacts_come_from_prepare(self):
        for job in self.workflow['jobs'].values():
            for step in job['steps']:
                action = step.get('uses')
                if action:
                    self.assertRegex(action, r'^[^@]+@[0-9a-f]{40}$')
        test_actions = [step.get('uses', '') for step in self.workflow['jobs']['test']['steps']]
        self.assertFalse(any(action.startswith('actions/upload-artifact@') for action in test_actions))


if __name__ == '__main__':
    unittest.main()
