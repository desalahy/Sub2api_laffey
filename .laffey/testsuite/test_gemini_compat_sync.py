from __future__ import annotations

import importlib.util
import json
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("laffey_sync", ROOT / ".laffey/tools/laffey_sync.py")
assert SPEC is not None and SPEC.loader is not None
sync = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(sync)
BASE = "55ff4c44637615d983e4da1894712e7831218a9f"
OWNED = [
    "backend/internal/service/gemini_tool_calls_laffey.go",
    "backend/internal/service/gemini_tool_calls_laffey_test.go",
    "backend/internal/service/gemini_tool_stream_laffey_test.go",
    "backend/internal/service/gemini_response_accumulator_laffey.go",
    "backend/internal/service/gemini_response_accumulator_laffey_test.go",
    "backend/internal/handler/gemini_stream_usage_laffey_test.go",
]


class GeminiCompatSyncTests(unittest.TestCase):
    def setUp(self) -> None:
        self.manifest = json.loads((ROOT / ".laffey/brand.json").read_text(encoding="utf-8"))
        self.rules = [r for r in self.manifest["replacements"] if str(r.get("find_file", "")).startswith("transforms/gemini_")]

    def test_overlay_matches_runtime(self) -> None:
        for path in OWNED:
            with self.subTest(path=path):
                self.assertEqual((ROOT / path).read_bytes(), (ROOT / ".laffey/overlay" / path).read_bytes())
                self.assertIn(path, self.manifest["allowlist"])

    def test_rebuild_is_exact_and_idempotent(self) -> None:
        self.assertTrue(self.rules, "Gemini changes need persistent transforms")
        with tempfile.TemporaryDirectory() as directory:
            candidate = Path(directory)
            targets = sorted({rule["path"] for rule in self.rules})
            for path in targets:
                # Before fragments must contain enough original context to
                # reconstruct each target in a source archive without Git.
                target = candidate / path
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes((ROOT / path).read_bytes())
            for rule in reversed(self.rules):
                target = candidate / rule["path"]
                before = (ROOT / ".laffey" / rule["find_file"]).read_text(encoding="utf-8")
                after = (ROOT / ".laffey" / rule["replace_file"]).read_text(encoding="utf-8")
                text = target.read_text(encoding="utf-8")
                self.assertEqual(text.count(after), rule["expected_count"])
                target.write_text(text.replace(after, before), encoding="utf-8", newline="\n")
            # With Git history available, compare the reconstructed input with
            # the independently pinned baseline, not only the runtime output.
            available = subprocess.run(["git", "cat-file", "-e", BASE], cwd=ROOT, capture_output=True).returncode == 0
            if available:
                for path in targets:
                    original = subprocess.check_output(["git", "show", f"{BASE}:{path}"], cwd=ROOT).decode("utf-8")
                    self.assertEqual(original, (candidate / path).read_text(encoding="utf-8"))
            manifest = {"replacements": self.rules}
            first = sync.apply_branding(candidate, ROOT / ".laffey/overlay", manifest)
            self.assertEqual(first["replacement_count"], len(self.rules))
            for path in targets + OWNED:
                self.assertEqual((ROOT / path).read_bytes(), (candidate / path).read_bytes())
                self.assertIn(path, self.manifest["allowlist"])
            hashes = sync.hash_tree(candidate)
            second = sync.apply_branding(candidate, ROOT / ".laffey/overlay", manifest)
            self.assertEqual(second["replacement_count"], 0)
            self.assertEqual(hashes, sync.hash_tree(candidate))

    def test_context_drift_blocks_sync(self) -> None:
        self.assertTrue(self.rules)
        for rule in self.rules:
            with self.subTest(path=rule["path"]), tempfile.TemporaryDirectory() as directory:
                candidate = Path(directory)
                target = candidate / rule["path"]
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("upstream context changed\n", encoding="utf-8")
                with self.assertRaises(sync.BrandingError):
                    sync._apply_replacement(candidate, rule, ROOT / ".laffey")


if __name__ == "__main__":
    unittest.main()
