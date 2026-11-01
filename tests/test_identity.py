import json
from pathlib import Path
import sys
import tempfile
import unittest
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
from initialize import initialize
from ci_config import load_config


class UuidTestIdentityTests(unittest.TestCase):
    def test_initialized_identity_and_selected_source(self):
        config = load_config(ROOT, initialized=True, repository="miyako/UuidTest")
        self.assertEqual(
            {key: config[key] for key in ("repository", "namespace", "cli", "language")},
            {"repository": "UuidTest", "namespace": "UuidTest", "cli": "uuid-test", "language": "go"},
        )
        self.assertEqual([path.name for path in (ROOT / "component").iterdir()], ["UuidTest"])
        self.assertFalse((ROOT / "cli-starters").exists())
        self.assertTrue((ROOT / "cli/go.mod").is_file())
        self.assertFalse((ROOT / "cli/Cargo.toml").exists())
        self.assertFalse((ROOT / "cli/CMakeLists.txt").exists())
        component = ROOT / "component/UuidTest"
        self.assertTrue((component / "Project/Sources/Classes/UUIDv7.4dm").is_file())
        self.assertFalse((component / "Project/Sources/Classes/Greeting.4dm").exists())
        catalog = ET.parse(component / "Project/Sources/catalog.4DCatalog").getroot()
        self.assertEqual(catalog.attrib["name"], "UuidTest")
        self.assertIn("cs.UuidTest.UUIDv7", (ROOT / "README.md").read_text())

    def test_adopted_initializer_noop_and_conflict(self):
        config = json.loads((ROOT / "template.json").read_text())
        with tempfile.TemporaryDirectory(prefix="uuid identity ") as temporary:
            root = Path(temporary)
            file = root / "template.json"
            file.write_text(json.dumps(config))
            before = file.read_bytes()
            args = {key: config[key] for key in ("repository", "namespace", "cli", "language", "feature")}
            self.assertFalse(initialize(root, **args))
            self.assertEqual(file.read_bytes(), before)
            with self.assertRaisesRegex(ValueError, "already initialized"):
                initialize(root, **dict(args, namespace="AnotherNamespace"))
            self.assertEqual(file.read_bytes(), before)
