import os
from pathlib import Path
import subprocess
import sys
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
import ci_config
import release_ref


class ReleaseRefTests(unittest.TestCase):
    sha = "a" * 40

    def git_result(self, status=0, output=None):
        return subprocess.CompletedProcess([], status, self.sha + "\n" if output is None else output, "")

    def test_existing_manual_tag_pins_source_not_dispatch_branch(self):
        with patch.object(release_ref.subprocess, "run", return_value=self.git_result()) as git:
            result = release_ref.resolve(ROOT, event="workflow_dispatch", version_tag="v0.1.0",
                                         ref_type="branch", ref_name="main", event_sha="b" * 40)
        self.assertEqual(result, {"tag": "v0.1.0", "sha": self.sha})
        self.assertEqual(git.call_args.args[0],
                         ["git", "rev-parse", "--verify", "refs/tags/v0.1.0^{commit}"])

    def test_push_tag_must_match_event_commit(self):
        with patch.object(release_ref.subprocess, "run", return_value=self.git_result()):
            self.assertEqual(release_ref.resolve(ROOT, event="push", ref_type="tag",
                                                ref_name="v0.1.0", event_sha=self.sha)["sha"], self.sha)
            with self.assertRaisesRegex(ValueError, "moved"):
                release_ref.resolve(ROOT, event="push", ref_type="tag",
                                    ref_name="v0.1.0", event_sha="b" * 40)

    def test_arbitrary_refs_rejected_before_git(self):
        with patch.object(release_ref.subprocess, "run") as git:
            for tag in ("main", "v0.1.0^{commit}", "../v0.1.0", "v01.1.0", "v0.1.0\n"):
                with self.subTest(tag=tag), self.assertRaises(ValueError):
                    release_ref.tag_commit(ROOT, tag)
            with self.assertRaisesRegex(ValueError, "tag"):
                release_ref.resolve(ROOT, event="push", ref_type="branch", ref_name="main")
            with self.assertRaises(ValueError):
                release_ref.resolve(ROOT, event="pull_request")
        git.assert_not_called()

    def test_missing_tag_and_invalid_sha_fail_closed(self):
        for result in (self.git_result(status=1), self.git_result(output="not a sha"),
                       self.git_result(output="A" * 40)):
            with patch.object(release_ref.subprocess, "run", return_value=result), self.assertRaises(ValueError):
                release_ref.tag_commit(ROOT, "v0.1.0")
        with self.assertRaisesRegex(ValueError, "full normalized"):
            release_ref.verify(ROOT, "v0.1.0", "abc123")

    def test_reverify_detects_tag_movement(self):
        with patch.object(release_ref.subprocess, "run", return_value=self.git_result()):
            self.assertEqual(release_ref.verify(ROOT, "v0.1.0", self.sha), self.sha)
        with patch.object(release_ref.subprocess, "run", return_value=self.git_result(output="b" * 40)):
            with self.assertRaisesRegex(ValueError, "moved"):
                release_ref.verify(ROOT, "v0.1.0", self.sha)

    def test_release_validation_accepts_explicit_tag_from_branch_dispatch(self):
        environment = {"GITHUB_REPOSITORY": "miyako/UuidTest", "GITHUB_REF_NAME": "main",
                       "FOURD_ENABLED": "true"}
        with patch.dict(os.environ, environment, clear=True), patch.object(
                sys, "argv", ["ci_config.py", "validate", "--release", "--tag", "v0.1.0"]):
            ci_config.main()
        with patch.dict(os.environ, environment, clear=True), patch.object(
                sys, "argv", ["ci_config.py", "validate", "--release"]):
            with self.assertRaises(SystemExit):
                ci_config.main()
