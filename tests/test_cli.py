import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import unittest
import uuid


ROOT = Path(__file__).resolve().parents[1]
UUID_ERROR = (
    b'{"protocol":1,"error":{"code":"invalid_arguments","message":'
    b'"expected generate, --version or --help"}}\n'
)


def json_line(value):
    return (json.dumps(value, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8")


class GoCLI(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        selected = os.environ.get("TEST_CLI_LANGUAGE")
        if selected == "none":
            raise unittest.SkipTest("CLI toolchain disabled by TEST_CLI_LANGUAGE=none")
        if selected not in (None, "go"):
            raise RuntimeError("TEST_CLI_LANGUAGE must be none or go")
        config = json.loads((ROOT / "template.json").read_text(encoding="utf-8"))
        if not config.get("initialized") or config.get("language") != "go":
            raise RuntimeError("template.json must configure initialized Go UUID CLI")
        original = ROOT / "cli"
        if not original.is_dir():
            raise RuntimeError("missing CLI source directory: " + str(original))
        if shutil.which("go") is None:
            raise RuntimeError("configured Go toolchain unavailable: go")

        scratch = tempfile.TemporaryDirectory(prefix=".cli-test path ", dir=Path(__file__).parent)
        cls.addClassCleanup(scratch.cleanup)
        root = Path(scratch.name)
        source = root / "source"
        shutil.copytree(original, source)
        executable_name = config["cli"] + (".exe" if os.name == "nt" else "")
        directory_name = "executables with spaces 世界"
        if os.name == "posix":
            directory_name += ' "quotes"'
        execution_directory = root / directory_name
        execution_directory.mkdir()
        cls.executable = execution_directory / executable_name
        work = root / "work"
        work.mkdir()
        environment = os.environ.copy()
        environment.pop("GOOS", None)
        environment.pop("GOARCH", None)
        environment.update(
            CGO_ENABLED="0",
            TMPDIR=str(work),
            GOCACHE=str(root / "go-cache"),
            GOPATH=str(root / "go-path"),
            GOTOOLCHAIN="local",
            GOPROXY="off",
            GOSUMDB="off",
            GOTMPDIR=str(work),
        )
        for command in [
            ["go", "test", "./..."],
            ["go", "build", "-o", str(cls.executable), "."],
        ]:
            result = subprocess.run(
                command, cwd=source, env=environment, capture_output=True, timeout=180
            )
            if result.returncode:
                raise RuntimeError(
                    "Go validation failed: " + repr(command) + "\n"
                    + result.stdout.decode("utf-8", errors="replace")
                    + result.stderr.decode("utf-8", errors="replace")
                )

    def assert_result(self, arguments, status, stdout, stderr):
        result = subprocess.run(
            [os.fspath(self.executable), *arguments], capture_output=True, timeout=10
        )
        self.assertEqual(result.returncode, status)
        self.assertEqual(result.stdout, stdout)
        self.assertEqual(result.stderr, stderr)

    def test_version(self):
        self.assert_result(["--version"], 0, b'{"protocol":1,"version":"0.1.0"}\n', b"")

    def test_help(self):
        self.assert_result(
            ["--help"], 0,
            b'{"protocol":1,"usage":"uuid-test generate | --version | --help"}\n',
            b"",
        )

    def test_generate(self):
        self.assertIn(" ", str(self.executable))
        self.assertIn("世界", str(self.executable))
        if os.name == "posix":
            self.assertIn('"', str(self.executable))
        seen = set()
        for _ in range(64):
            before = time.time_ns() // 1_000_000
            result = subprocess.run(
                [os.fspath(self.executable), "generate"], capture_output=True, timeout=10
            )
            after = time.time_ns() // 1_000_000
            self.assertEqual(result.returncode, 0)
            self.assertEqual(result.stderr, b"")
            value = json.loads(result.stdout)
            self.assertEqual(set(value), {"protocol", "uuid"})
            self.assertEqual(value["protocol"], 1)
            self.assertIs(type(value["protocol"]), int)
            self.assertRegex(
                value["uuid"],
                r"^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$",
            )
            self.assertEqual(result.stdout, json_line(value))
            parsed = uuid.UUID(value["uuid"])
            self.assertEqual(parsed.version, 7)
            self.assertEqual(parsed.variant, uuid.RFC_4122)
            milliseconds = int.from_bytes(parsed.bytes[:6], "big")
            self.assertGreaterEqual(milliseconds, before)
            self.assertLessEqual(milliseconds, after)
            self.assertNotIn(value["uuid"], seen)
            seen.add(value["uuid"])

    def test_invalid_arguments(self):
        cases = [
            [], [""], ["unknown"], ["Generate"], ["GENERATE"], ["generate "],
            ["generate", "--name", "World"], ["generate", "extra"],
            ["generate", "generate"], ["--generate"],
            ["--version", "extra"], ["--version", "--version"], ["--help", "extra"],
            ["--help", "--help"], ["--help", "--version"], ["--version", "generate"],
            ["generate", '--help "世界" \\backslash; $(echo nope)'],
            ["世界 café 😀"], ["line\nbreak"], ["\t"], ["--help=generate"],
        ]
        for arguments in cases:
            with self.subTest(arguments=arguments):
                self.assert_result(arguments, 2, b"", UUID_ERROR)

    @unittest.skipUnless(os.name == "posix", "raw byte arguments require POSIX")
    def test_malformed_utf8(self):
        for value in [
            b"\x80", b"\xff", b"\xc0\xaf", b"\xc2", b"\xe2\x82",
            b"\xed\xa0\x80", b"\xf4\x90\x80\x80", b"generate\xfe",
        ]:
            for arguments in ([value], [b"generate", value], [b"--help", value]):
                with self.subTest(arguments=arguments):
                    self.assert_result(arguments, 2, b"", UUID_ERROR)


if __name__ == "__main__":
    unittest.main()
