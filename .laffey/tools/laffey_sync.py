#!/usr/bin/env python3
from __future__ import annotations

import argparse
import fnmatch
import hashlib
import io
import json
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
from pathlib import Path, PurePosixPath
from typing import Any, Iterable, Mapping, Sequence


UPSTREAM_TAG_RE = re.compile(r"^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$")


class BrandingError(RuntimeError):
    pass


def parse_upstream_tag(tag: str) -> tuple[int, int, int]:
    match = UPSTREAM_TAG_RE.fullmatch(tag)
    if match is None:
        raise ValueError(f"Invalid stable upstream tag: {tag}")
    return tuple(int(part) for part in match.groups())  # type: ignore[return-value]


def build_laffey_release_tag(upstream_tag: str, revision: int) -> str:
    parse_upstream_tag(upstream_tag)
    if revision < 1:
        raise ValueError("Laffey release revision must be at least 1")
    return f"{upstream_tag}-laffey.{revision}"


def select_latest_release(releases: Sequence[Mapping[str, Any]]) -> Mapping[str, Any]:
    stable: list[tuple[tuple[int, int, int], Mapping[str, Any]]] = []
    for release in releases:
        if release.get("draft") or release.get("prerelease"):
            continue
        tag = str(release.get("tag_name", ""))
        try:
            version = parse_upstream_tag(tag)
        except ValueError:
            continue
        stable.append((version, release))
    if not stable:
        raise BrandingError("No stable upstream release was found")
    return max(stable, key=lambda item: item[0])[1]


def run_git(repo: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=repo,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def fetch_release_ref(repo: Path, remote: str, tag: str) -> str:
    parse_upstream_tag(tag)
    isolated_ref = f"refs/remotes/upstream-release/{tag}"
    run_git(
        repo,
        "fetch",
        "--no-tags",
        remote,
        f"+refs/tags/{tag}:{isolated_ref}",
    )
    return run_git(repo, "rev-list", "-n", "1", isolated_ref)


def load_manifest(path: Path) -> dict[str, Any]:
    manifest = json.loads(path.read_text(encoding="utf-8"))
    if manifest.get("schema_version") != 1:
        raise BrandingError("brand.json must use schema_version 1")
    for field in ("brand", "allowlist", "replacements"):
        if field not in manifest:
            raise BrandingError(f"brand.json is missing {field}")
    return manifest


def write_json(path: Path, value: Mapping[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(value, ensure_ascii=True, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
        newline="\n",
    )


def build_lock(
    *,
    repository: str,
    tag: str,
    commit: str,
    base_commit: str,
) -> dict[str, Any]:
    parse_upstream_tag(tag)
    for name, value in (("commit", commit), ("base_commit", base_commit)):
        if re.fullmatch(r"[0-9a-f]{40}", value) is None:
            raise ValueError(f"{name} must be a 40-character lowercase Git SHA")
    return {
        "schema_version": 1,
        "upstream": {
            "repository": repository,
            "tag": tag,
            "commit": commit,
        },
        "base_commit": base_commit,
    }


def _copy_overlay(overlay_dir: Path, candidate_dir: Path) -> int:
    if not overlay_dir.exists():
        raise BrandingError(f"Overlay directory does not exist: {overlay_dir}")
    copied = 0
    for source in sorted(overlay_dir.rglob("*")):
        if source.is_dir():
            continue
        relative = source.relative_to(overlay_dir)
        destination = candidate_dir / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        if source.is_symlink():
            if destination.exists() or destination.is_symlink():
                destination.unlink()
            destination.symlink_to(source.readlink())
        else:
            shutil.copy2(source, destination)
        copied += 1
    return copied


def _copy_preserved_paths(repo: Path, candidate_dir: Path, paths: Iterable[str]) -> int:
    copied = 0
    repo_root = repo.resolve()
    for relative_value in paths:
        relative = PurePosixPath(str(relative_value))
        source = (repo_root / Path(*relative.parts)).resolve()
        try:
            source.relative_to(repo_root)
        except ValueError as error:
            raise BrandingError(f"Preserved path escapes the repository: {relative}") from error
        if not source.exists() and not source.is_symlink():
            raise BrandingError(f"Preserved path does not exist: {relative}")
        destination = candidate_dir / Path(*relative.parts)
        destination.parent.mkdir(parents=True, exist_ok=True)
        if source.is_dir():
            shutil.copytree(source, destination, dirs_exist_ok=True)
        elif source.is_symlink():
            if destination.exists() or destination.is_symlink():
                destination.unlink()
            destination.symlink_to(source.readlink())
        else:
            shutil.copy2(source, destination)
        copied += 1
    return copied


def _rule_text(rule: Mapping[str, Any], inline_key: str, file_key: str, rule_root: Path) -> str:
    if inline_key in rule:
        return str(rule[inline_key])
    if file_key not in rule:
        raise BrandingError(f"Replacement rule is missing {inline_key} or {file_key}")
    path = (rule_root / str(rule[file_key])).resolve()
    try:
        path.relative_to(rule_root.resolve())
    except ValueError as error:
        raise BrandingError(f"Replacement fragment escapes .laffey: {path}") from error
    if not path.is_file():
        raise BrandingError(f"Replacement fragment does not exist: {path}")
    return path.read_text(encoding="utf-8")


def _apply_replacement(
    candidate_dir: Path,
    rule: Mapping[str, Any],
    rule_root: Path,
) -> bool:
    relative_path = str(rule["path"])
    target = candidate_dir / relative_path
    if not target.is_file():
        raise BrandingError(f"Replacement target does not exist: {relative_path}")

    source = _rule_text(rule, "find", "find_file", rule_root)
    replacement = _rule_text(rule, "replace", "replace_file", rule_root)
    expected = int(rule.get("expected_count", 1))
    if expected < 1 or not source or source == replacement:
        raise BrandingError(f"Invalid replacement rule for {relative_path}")

    content = target.read_text(encoding="utf-8")
    unbranded_parts = content.split(replacement)
    replacement_count = len(unbranded_parts) - 1
    source_count = sum(part.count(source) for part in unbranded_parts)
    if source_count == expected:
        target.write_text(
            replacement.join(part.replace(source, replacement) for part in unbranded_parts),
            encoding="utf-8",
            newline="\n",
        )
        return True
    if source_count == 0 and replacement_count >= expected:
        return False
    raise BrandingError(
        f"Replacement context drifted in {relative_path}: "
        f"expected {expected} source occurrence(s), found {source_count}"
    )


def apply_branding(
    candidate_dir: Path,
    overlay_dir: Path,
    manifest: Mapping[str, Any],
) -> dict[str, Any]:
    copied = _copy_overlay(overlay_dir, candidate_dir)
    replacements = 0
    for rule in manifest.get("replacements", []):
        if _apply_replacement(candidate_dir, rule, overlay_dir.parent):
            replacements += 1
    return {"copied_file_count": copied, "replacement_count": replacements}


def _file_digest(path: Path) -> str:
    if path.is_symlink():
        return f"symlink:{path.readlink().as_posix()}"
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def hash_tree(root: Path) -> dict[str, str]:
    hashes: dict[str, str] = {}
    for path in sorted(root.rglob("*")):
        if path.is_dir() or ".git" in path.relative_to(root).parts:
            continue
        relative = path.relative_to(root).as_posix()
        hashes[relative] = _file_digest(path)
    return hashes


def _is_allowed(path: str, patterns: Iterable[str]) -> bool:
    normalized = PurePosixPath(path).as_posix()
    for pattern in patterns:
        pattern = PurePosixPath(pattern).as_posix()
        if pattern.endswith("/**"):
            prefix = pattern[:-3].rstrip("/")
            if normalized == prefix or normalized.startswith(prefix + "/"):
                return True
        elif fnmatch.fnmatchcase(normalized, pattern):
            return True
    return False


def verify_candidate(
    upstream_dir: Path,
    candidate_dir: Path,
    allowlist: Iterable[str],
) -> dict[str, Any]:
    upstream_hashes = hash_tree(upstream_dir)
    candidate_hashes = hash_tree(candidate_dir)
    changed = sorted(
        path
        for path in upstream_hashes.keys() | candidate_hashes.keys()
        if upstream_hashes.get(path) != candidate_hashes.get(path)
    )
    unexpected = [path for path in changed if not _is_allowed(path, allowlist)]
    report = {"changed_paths": changed, "unexpected_paths": unexpected}
    if unexpected:
        raise BrandingError(
            "Candidate differs from upstream outside the brand allowlist: "
            + ", ".join(unexpected)
        )
    return report


def render_report(lock: Mapping[str, Any], verification: Mapping[str, Any]) -> str:
    upstream = lock["upstream"]
    changed_paths = list(verification.get("changed_paths", []))
    lines = [
        "# Laffey Upstream Sync Report",
        "",
        f"- Upstream repository: `{upstream['repository']}`",
        f"- Upstream tag: `{upstream['tag']}`",
        f"- Upstream commit: `{upstream['commit']}`",
        f"- Base commit: `{lock['base_commit']}`",
        f"- Allowlisted brand differences: `{len(changed_paths)}`",
        "",
        "## Brand Difference Paths",
        "",
    ]
    lines.extend(f"- `{path}`" for path in changed_paths)
    lines.append("")
    return "\n".join(lines)


def _extract_archive(repo: Path, ref: str, destination: Path, pathspec: str | None = None) -> None:
    command = ["git", "archive", "--format=tar", ref]
    if pathspec is not None:
        command.append(pathspec)
    result = subprocess.run(command, cwd=repo, check=True, capture_output=True)
    with tarfile.open(fileobj=io.BytesIO(result.stdout), mode="r:") as archive:
        archive.extractall(destination, filter="data")


def build_candidate(
    *,
    repo: Path,
    upstream_ref: str,
    base_ref: str,
    upstream_repository: str,
    upstream_tag: str,
    output_dir: Path,
) -> dict[str, Any]:
    upstream_commit = run_git(repo, "rev-list", "-n", "1", upstream_ref)
    base_commit = run_git(repo, "rev-parse", base_ref)
    lock = build_lock(
        repository=upstream_repository,
        tag=upstream_tag,
        commit=upstream_commit,
        base_commit=base_commit,
    )

    if output_dir.exists() and any(output_dir.iterdir()):
        raise BrandingError(f"Candidate output directory must be empty: {output_dir}")
    output_dir.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(prefix="laffey-upstream-") as temporary:
        upstream_dir = Path(temporary) / "upstream"
        upstream_dir.mkdir()
        _extract_archive(repo, upstream_ref, upstream_dir)
        shutil.copytree(upstream_dir, output_dir, dirs_exist_ok=True)
        brand_source = repo / ".laffey"
        if not brand_source.is_dir():
            raise BrandingError(f"Trusted brand source does not exist: {brand_source}")
        shutil.copytree(
            brand_source,
            output_dir / ".laffey",
            dirs_exist_ok=True,
            ignore=shutil.ignore_patterns(
                "__pycache__",
                "node_modules",
                "test-results",
                "*.pyc",
                "*.pyo",
            ),
        )

        manifest_path = output_dir / ".laffey" / "brand.json"
        manifest = load_manifest(manifest_path)
        _copy_preserved_paths(repo, output_dir, manifest.get("preserve_paths", []))
        overlay_dir = output_dir / ".laffey" / "overlay"
        apply_result = apply_branding(output_dir, overlay_dir, manifest)
        first_hashes = hash_tree(output_dir)
        second_apply = apply_branding(output_dir, overlay_dir, manifest)
        if second_apply["replacement_count"] != 0 or hash_tree(output_dir) != first_hashes:
            raise BrandingError("Branding overlay is not idempotent")

        write_json(output_dir / ".laffey" / "upstream.lock.json", lock)
        verification = verify_candidate(upstream_dir, output_dir, manifest["allowlist"])
        report = render_report(lock, verification)
        (output_dir / ".laffey" / "generated-report.md").write_text(
            report,
            encoding="utf-8",
            newline="\n",
        )

    return {
        "lock": lock,
        "apply": apply_result,
        "verification": verification,
        "report": report,
    }


def fetch_release_list(repository: str) -> list[dict[str, Any]]:
    url = f"https://api.github.com/repos/{repository}/releases?per_page=100"
    request = urllib.request.Request(
        url,
        headers={"Accept": "application/vnd.github+json", "User-Agent": "laffey-sync"},
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        payload = json.load(response)
    if not isinstance(payload, list):
        raise BrandingError("GitHub releases response was not a list")
    return payload


def _command_latest(args: argparse.Namespace) -> int:
    release = select_latest_release(fetch_release_list(args.repository))
    print(json.dumps(release, ensure_ascii=True, sort_keys=True))
    return 0


def _command_fetch(args: argparse.Namespace) -> int:
    commit = fetch_release_ref(Path(args.repo), args.remote, args.tag)
    print(commit)
    return 0


def _command_build(args: argparse.Namespace) -> int:
    result = build_candidate(
        repo=Path(args.repo).resolve(),
        upstream_ref=args.upstream_ref,
        base_ref=args.base_ref,
        upstream_repository=args.upstream_repository,
        upstream_tag=args.upstream_tag,
        output_dir=Path(args.output).resolve(),
    )
    print(json.dumps(result, ensure_ascii=True, sort_keys=True))
    return 0


def _command_release_tag(args: argparse.Namespace) -> int:
    print(build_laffey_release_tag(args.upstream_tag, args.revision))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Build verified Laffey snapshots from upstream releases")
    subparsers = parser.add_subparsers(dest="command", required=True)

    latest = subparsers.add_parser("latest", help="Resolve the latest stable upstream GitHub release")
    latest.add_argument("--repository", default="Wei-Shaw/sub2api")
    latest.set_defaults(handler=_command_latest)

    fetch = subparsers.add_parser("fetch", help="Fetch an upstream tag into an isolated remote ref")
    fetch.add_argument("--repo", default=".")
    fetch.add_argument("--remote", default="upstream")
    fetch.add_argument("--tag", required=True)
    fetch.set_defaults(handler=_command_fetch)

    build = subparsers.add_parser("build", help="Build and verify a branded candidate tree")
    build.add_argument("--repo", default=".")
    build.add_argument("--upstream-ref", required=True)
    build.add_argument("--base-ref", required=True)
    build.add_argument("--upstream-repository", default="Wei-Shaw/sub2api")
    build.add_argument("--upstream-tag", required=True)
    build.add_argument("--output", required=True)
    build.set_defaults(handler=_command_build)

    release_tag = subparsers.add_parser("release-tag", help="Build a Laffey namespaced release tag")
    release_tag.add_argument("--upstream-tag", required=True)
    release_tag.add_argument("--revision", type=int, required=True)
    release_tag.set_defaults(handler=_command_release_tag)
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        return int(args.handler(args))
    except (BrandingError, ValueError, subprocess.CalledProcessError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
