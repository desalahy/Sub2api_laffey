# Gemini Thinking Effort Preservation Implementation Plan

> Execution: native implementation; all Go formatting, tests, lint and release builds run in GitHub Actions. User explicitly authorized direct submission and packaging.

**Goal:** Preserve explicit Gemini 4 thinking levels through Chat/Messages conversion so Hydrogen `xhigh` and Carbon `high` reach the upstream unchanged.

**Architecture:** Keep the client's original Chat effort in the Gemini-specific adapter before the shared Anthropic bridge loses the distinction between `xhigh` and `max`. The existing Gemini thinking converter preserves explicit Gemini 4 levels while retaining legacy Gemini 2.5/3 behavior. Usage records report the actual forwarded level, including `xhigh` and `max`.

**Tech Stack:** Go 1.27.3, existing gateway services, Python synchronization contracts, GitHub Actions/GoReleaser.

**Spec:** User-approved fix based on direct o10 native tests and deployed requested-versus-forwarded usage records; evidence in the session ledger.

## Constraints

- No local Go execution, compilation, tests, cache access or cache cleanup.
- No production deployment or client configuration changes in this delivery.
- Preserve tool-call compatibility, native endpoints, legacy thinking budgets and suffix-routed models.
- Keep unsupported explicit Gemini 4 levels visible to upstream validation instead of silently changing user intent.
- Repository guidance favors RelayKit for new translators; this fix repairs parameter fidelity in the existing Gemini adapter and adds no translator or route.
- Persist source edits and new tests through `.laffey` exact transforms and overlay.

## Review focus

- `xhigh` versus `max` must remain distinct across the intermediate Anthropic bridge.
- Model mappings must use the actual upstream model when selecting conversion behavior.
- Omitted effort must remain omitted; disabled thinking and suffix-routed models keep existing behavior.
- Gemini 2.5/3 compatibility and billing metadata must remain coherent.
- Stream, nonstream and signature-retry paths must use the same original intent.

## Tasks

- [ ] Add real forwarding tests asserting upstream Gemini bodies and returned usage effort for Chat/Messages, including mapped models, legacy compatibility and signature retry. Commit and observe failure in GitHub Actions before source changes.
- [ ] Preserve original Chat effort in `gemini_chat_completions_compat_service.go`; preserve Gemini 4 levels in `gemini_compat_thinking.go`; recognize forwarded xhigh/max in `gemini_reasoning_effort.go`. Rebuild sync transforms/overlay and run Python contracts.
- [ ] Run remote focused tests, full Linux unit/integration tests, race tests and lint. Independent review; address findings before merge.
- [ ] Merge exact validated head and dispatch GitHub Release for `v0.2.20-laffey.3`. Verify stable/latest release, Linux amd64 archive, checksum and GHCR image revision.
