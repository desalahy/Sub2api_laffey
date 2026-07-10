# Laffey upstream snapshot workflow

This directory is the trusted branding layer for rebuilding Laffey API from a
stable `Wei-Shaw/sub2api` release. Product files outside the allowlist come
from the upstream release archive byte-for-byte.

## Directory contract

- `brand.json` defines the brand values, preserved workflow files, exact
  replacements, and the only paths allowed to differ from upstream.
- `overlay/` contains Laffey-owned assets, pages, documentation, and small
  source additions. Its tree is copied over the upstream archive.
- `transforms/` stores reviewable before/after fragments for replacements that
  are too large for `brand.json`.
- `tools/laffey_sync.py` resolves releases, fetches tags into isolated refs,
  builds candidates, applies the brand layer, and verifies the allowlist.
- `upstream.lock.json` is `bootstrap` until the first generated snapshot is
  merged. Generated candidates replace it with the exact upstream tag and
  commit plus the `origin/master` base commit.

## Local verification

Run the sync contract tests:

```powershell
python -m unittest discover -s .laffey/testsuite -v
```

Fetch a release without touching a same-named local tag:

```powershell
python .laffey/tools/laffey_sync.py fetch --repo . --remote upstream --tag v0.1.149
```

Build a candidate in an empty directory outside the worktree:

```powershell
python .laffey/tools/laffey_sync.py build `
  --repo . `
  --upstream-ref refs/remotes/upstream-release/v0.1.149 `
  --base-ref origin/master `
  --upstream-tag v0.1.149 `
  --output $env:TEMP/laffey-v0.1.149
```

The command fails if an exact source fragment drifted, branding is not
idempotent, or any file outside the allowlist differs from upstream.

The browser smoke package is independently locked under `.laffey/browser`:

```powershell
npm ci --prefix .laffey/browser
& .laffey/browser/node_modules/.bin/playwright.cmd install chromium
$env:LAFFEY_PREVIEW_URL = "http://127.0.0.1:3000"
npm test --prefix .laffey/browser
```

When the Playwright CDN is unavailable locally, set
`PLAYWRIGHT_EXECUTABLE_PATH` to an installed Chrome or Edge executable before
running the tests. CI leaves it unset and uses the pinned Playwright browser.

## GitHub Actions

`sync-upstream.yml` runs daily at 01:00 UTC (09:00 China Standard Time) and can
also be dispatched with a stable upstream tag. The candidate job has contents
write permission but does not execute upstream code. Backend, frontend,
contract, and browser tests run later with read-only tokens.

Successful validation creates or updates `sync/upstream-<tag>` and opens a
manual PR. Failure leaves `master` unchanged, publishes the
`laffey-sync/validated` failure status, and creates or updates one blocking
issue for that upstream tag.

Enable **Allow GitHub Actions to create and approve pull requests** in the
repository Actions settings. The workflow creates PRs but never approves or
merges them.

## Releases

Only `vX.Y.Z-laffey.N` tags are accepted. A manual Release workflow dispatch
may create the requested tag when absent. An existing tag is reusable only
when it points to current `master`, and `vX.Y.Z` must equal the upstream tag in
`upstream.lock.json`. The workflow publishes artifacts and images; it does not
deploy a server.
