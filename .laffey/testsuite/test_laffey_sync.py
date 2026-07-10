from __future__ import annotations

import importlib.util
import json
import subprocess
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).parents[1] / "tools" / "laffey_sync.py"
SPEC = importlib.util.spec_from_file_location("laffey_sync", MODULE_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"Unable to load {MODULE_PATH}")
laffey_sync = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(laffey_sync)


def run_git(repo: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=repo,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


class ReleaseSelectionTests(unittest.TestCase):
    def test_selects_latest_stable_upstream_release(self) -> None:
        releases = [
            {"tag_name": "v1.4.0-rc.1", "draft": False, "prerelease": True},
            {"tag_name": "v1.3.0", "draft": True, "prerelease": False},
            {"tag_name": "v1.2.4", "draft": False, "prerelease": False},
            {"tag_name": "v1.2.3", "draft": False, "prerelease": False},
        ]

        selected = laffey_sync.select_latest_release(releases)

        self.assertEqual(selected["tag_name"], "v1.2.4")

    def test_upstream_tag_rejects_laffey_or_prerelease_suffixes(self) -> None:
        self.assertEqual(laffey_sync.parse_upstream_tag("v1.2.3"), (1, 2, 3))

        for invalid in ("1.2.3", "v1.2", "v1.2.3-laffey.1", "v1.2.3-rc.1"):
            with self.subTest(invalid=invalid):
                with self.assertRaises(ValueError):
                    laffey_sync.parse_upstream_tag(invalid)

    def test_release_tag_uses_separate_laffey_namespace(self) -> None:
        self.assertEqual(
            laffey_sync.build_laffey_release_tag("v1.2.3", 4),
            "v1.2.3-laffey.4",
        )
        with self.assertRaises(ValueError):
            laffey_sync.build_laffey_release_tag("v1.2.3", 0)


class IsolatedFetchTests(unittest.TestCase):
    def test_fetches_upstream_tag_without_touching_same_named_local_tag(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            remote = root / "remote"
            clone = root / "clone"
            remote.mkdir()
            run_git(remote, "init")
            run_git(remote, "config", "user.name", "Test")
            run_git(remote, "config", "user.email", "test@example.com")
            (remote / "source.txt").write_text("upstream", encoding="utf-8")
            run_git(remote, "add", "source.txt")
            run_git(remote, "commit", "-m", "upstream")
            run_git(remote, "tag", "-a", "v1.2.3", "-m", "upstream release")
            upstream_commit = run_git(remote, "rev-parse", "HEAD")

            run_git(root, "clone", str(remote), str(clone))
            run_git(clone, "config", "user.name", "Test")
            run_git(clone, "config", "user.email", "test@example.com")
            run_git(clone, "tag", "-d", "v1.2.3")
            (clone / "fork.txt").write_text("fork", encoding="utf-8")
            run_git(clone, "add", "fork.txt")
            run_git(clone, "commit", "-m", "fork")
            run_git(clone, "tag", "v1.2.3")
            fork_tag_commit = run_git(clone, "rev-parse", "v1.2.3")

            fetched = laffey_sync.fetch_release_ref(
                clone,
                "origin",
                "v1.2.3",
            )

            self.assertEqual(fetched, upstream_commit)
            self.assertEqual(run_git(clone, "rev-parse", "v1.2.3"), fork_tag_commit)
            self.assertEqual(
                run_git(clone, "rev-list", "-n", "1", "refs/remotes/upstream-release/v1.2.3"),
                upstream_commit,
            )


class BrandingTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.root = Path(self.temp_dir.name)
        self.candidate = self.root / "candidate"
        self.upstream = self.root / "upstream"
        self.overlay = self.root / "overlay"
        self.candidate.mkdir()
        self.upstream.mkdir()
        self.overlay.mkdir()

        (self.upstream / "app.txt").write_text("name=Sub2API\n", encoding="utf-8")
        (self.upstream / "untouched.txt").write_text("same\n", encoding="utf-8")
        (self.candidate / "app.txt").write_text("name=Sub2API\n", encoding="utf-8")
        (self.candidate / "untouched.txt").write_text("same\n", encoding="utf-8")
        (self.overlay / "assets").mkdir()
        (self.overlay / "assets" / "logo.txt").write_text("laffey\n", encoding="utf-8")

        self.manifest = {
            "schema_version": 1,
            "brand": {
                "product_name": "Laffey API",
                "repository": "desalahy/Sub2api_laffey",
                "ghcr_image": "ghcr.io/desalahy/sub2api_laffey",
                "release_suffix": "laffey",
            },
            "allowlist": ["app.txt", "assets/**", ".laffey/**"],
            "replacements": [
                {
                    "path": "app.txt",
                    "find": "name=Sub2API",
                    "replace": "name=Laffey API",
                    "expected_count": 1,
                }
            ],
        }

    def test_applies_overlay_and_exact_replacement_idempotently(self) -> None:
        first = laffey_sync.apply_branding(
            self.candidate,
            self.overlay,
            self.manifest,
        )
        first_hashes = laffey_sync.hash_tree(self.candidate)

        second = laffey_sync.apply_branding(
            self.candidate,
            self.overlay,
            self.manifest,
        )

        self.assertEqual((self.candidate / "app.txt").read_text(encoding="utf-8"), "name=Laffey API\n")
        self.assertEqual((self.candidate / "assets" / "logo.txt").read_text(encoding="utf-8"), "laffey\n")
        self.assertEqual(first["replacement_count"], 1)
        self.assertEqual(second["replacement_count"], 0)
        self.assertEqual(laffey_sync.hash_tree(self.candidate), first_hashes)

    def test_exact_replacement_fails_when_upstream_context_drifts(self) -> None:
        (self.candidate / "app.txt").write_text("name=RenamedUpstream\n", encoding="utf-8")

        with self.assertRaisesRegex(laffey_sync.BrandingError, "app.txt"):
            laffey_sync.apply_branding(self.candidate, self.overlay, self.manifest)

    def test_replacement_can_load_reviewable_before_and_after_fragments(self) -> None:
        transforms = self.overlay.parent / "transforms"
        transforms.mkdir()
        (transforms / "before.txt").write_text("name=Sub2API", encoding="utf-8")
        (transforms / "after.txt").write_text("name=Laffey API", encoding="utf-8")
        self.manifest["replacements"] = [
            {
                "path": "app.txt",
                "find_file": "transforms/before.txt",
                "replace_file": "transforms/after.txt",
                "expected_count": 1,
            }
        ]

        laffey_sync.apply_branding(self.candidate, self.overlay, self.manifest)

        self.assertEqual((self.candidate / "app.txt").read_text(encoding="utf-8"), "name=Laffey API\n")

    def test_replacement_is_idempotent_when_after_fragment_retains_legacy_source(self) -> None:
        self.manifest["replacements"] = [
            {
                "path": "app.txt",
                "find": "name=Sub2API",
                "replace": "name=Laffey API\nlegacy=name=Sub2API",
                "expected_count": 1,
            }
        ]

        laffey_sync.apply_branding(self.candidate, self.overlay, self.manifest)
        first = (self.candidate / "app.txt").read_text(encoding="utf-8")
        second = laffey_sync.apply_branding(self.candidate, self.overlay, self.manifest)

        self.assertEqual((self.candidate / "app.txt").read_text(encoding="utf-8"), first)
        self.assertEqual(second["replacement_count"], 0)

    def test_verifier_rejects_changes_outside_allowlist(self) -> None:
        laffey_sync.apply_branding(self.candidate, self.overlay, self.manifest)
        (self.candidate / "untouched.txt").write_text("local feature\n", encoding="utf-8")

        with self.assertRaisesRegex(laffey_sync.BrandingError, "untouched.txt"):
            laffey_sync.verify_candidate(
                self.upstream,
                self.candidate,
                self.manifest["allowlist"],
            )

    def test_verifier_reports_only_allowlisted_brand_differences(self) -> None:
        laffey_sync.apply_branding(self.candidate, self.overlay, self.manifest)

        report = laffey_sync.verify_candidate(
            self.upstream,
            self.candidate,
            self.manifest["allowlist"],
        )

        self.assertEqual(report["unexpected_paths"], [])
        self.assertEqual(report["changed_paths"], ["app.txt", "assets/logo.txt"])

    def test_writes_stable_lock_file_and_markdown_report(self) -> None:
        lock = laffey_sync.build_lock(
            repository="Wei-Shaw/sub2api",
            tag="v1.2.3",
            commit="a" * 40,
            base_commit="b" * 40,
        )
        output = self.candidate / ".laffey" / "upstream.lock.json"
        laffey_sync.write_json(output, lock)
        report = laffey_sync.render_report(
            lock,
            {"changed_paths": ["app.txt"], "unexpected_paths": []},
        )

        self.assertEqual(json.loads(output.read_text(encoding="utf-8")), lock)
        self.assertIn("v1.2.3", report)
        self.assertIn("app.txt", report)
        self.assertNotIn("generated_at", lock)


class CandidateBuildTests(unittest.TestCase):
    def test_builds_from_annotated_release_commit_and_restores_brand_layer(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            repo = root / "repo"
            output = root / "candidate"
            repo.mkdir()
            run_git(repo, "init")
            run_git(repo, "config", "user.name", "Test")
            run_git(repo, "config", "user.email", "test@example.com")

            (repo / "app.txt").write_text("name=Sub2API\n", encoding="utf-8")
            run_git(repo, "add", "app.txt")
            run_git(repo, "commit", "-m", "upstream")
            upstream_commit = run_git(repo, "rev-parse", "HEAD")
            run_git(repo, "tag", "-a", "v1.2.3", "-m", "release")

            overlay = repo / ".laffey" / "overlay"
            overlay.mkdir(parents=True)
            (overlay / "README.md").write_text("# Laffey API\n", encoding="utf-8")
            manifest = {
                "schema_version": 1,
                "brand": {
                    "product_name": "Laffey API",
                    "repository": "desalahy/Sub2api_laffey",
                    "ghcr_image": "ghcr.io/desalahy/sub2api_laffey",
                    "release_suffix": "laffey",
                },
                "allowlist": [".laffey/**", "README.md", "app.txt"],
                "preserve_paths": [".github/workflows/sync-upstream.yml"],
                "replacements": [
                    {
                        "path": "app.txt",
                        "find": "name=Sub2API",
                        "replace": "name=Laffey API",
                        "expected_count": 1,
                    }
                ],
            }
            laffey_sync.write_json(repo / ".laffey" / "brand.json", manifest)
            workflow = repo / ".github" / "workflows" / "sync-upstream.yml"
            workflow.parent.mkdir(parents=True)
            workflow.write_text("name: trusted sync\n", encoding="utf-8")
            manifest["allowlist"].append(".github/workflows/sync-upstream.yml")
            laffey_sync.write_json(repo / ".laffey" / "brand.json", manifest)
            cache_dir = repo / ".laffey" / "tools" / "__pycache__"
            cache_dir.mkdir(parents=True)
            (cache_dir / "laffey_sync.pyc").write_bytes(b"cache")
            browser_modules = repo / ".laffey" / "browser" / "node_modules" / "fixture"
            browser_modules.mkdir(parents=True)
            (browser_modules / "index.js").write_text("generated dependency\n", encoding="utf-8")
            browser_results = repo / ".laffey" / "browser" / "test-results" / "fixture"
            browser_results.mkdir(parents=True)
            (browser_results / "screenshot.png").write_bytes(b"generated screenshot")
            base_commit = run_git(repo, "rev-parse", "HEAD")

            result = laffey_sync.build_candidate(
                repo=repo,
                upstream_ref="v1.2.3",
                base_ref=base_commit,
                upstream_repository="Wei-Shaw/sub2api",
                upstream_tag="v1.2.3",
                output_dir=output,
            )

            self.assertEqual(result["lock"]["upstream"]["commit"], upstream_commit)
            self.assertEqual((output / "app.txt").read_text(encoding="utf-8"), "name=Laffey API\n")
            self.assertEqual((output / "README.md").read_text(encoding="utf-8"), "# Laffey API\n")
            self.assertEqual(
                (output / ".github" / "workflows" / "sync-upstream.yml").read_text(encoding="utf-8"),
                "name: trusted sync\n",
            )
            self.assertTrue((output / ".laffey" / "generated-report.md").is_file())
            self.assertFalse((output / ".laffey" / "tools" / "__pycache__").exists())
            self.assertFalse((output / ".laffey" / "browser" / "node_modules").exists())
            self.assertFalse((output / ".laffey" / "browser" / "test-results").exists())


if __name__ == "__main__":
    unittest.main()
