# Gemini tool-call compatibility patch A

Base: `55ff4c44637615d983e4da1894712e7831218a9f` (upstream v0.2.20).

## Behavior

- Distinct same-name calls retain independent IDs/indexes, including identical arguments and calls without upstream IDs.
- String argument fragments are buffered per call; ordered parts preserve text arriving behind an incomplete call. Tool object numbers retain JSON precision.
- Gemini Chat/Messages streams reject incomplete or malformed tool responses without emitting success. Observed usage is retained, including through the Messages HTTP handler; failed streams do not manufacture an image charge.
- Buffered native OAuth responses preserve candidate identity, all tool calls, signed parts, images and terminal metadata. Repeated cumulative image payloads retain per-frame multiplicity rather than accumulating charges.
- Nonstream thought blocks and MAX_TOKENS retain their meanings.

Redis signature recovery and strict tool-choice/parallel constraint changes are patch B and are not included.

## Verification

- Real Chat/Messages handler tests reproduced same-name merging, lost identical calls and rounded integers before the fix.
- Collector/ForwardNative tests reproduced lost calls and mixed thought/text before the fix.
- Independent review produced additional regression tests for prefix fragments, anonymous call interleaving, multiple candidates, cumulative images, terminal errors, observed usage settlement and nonstream parity. Targeted regressions passed after fixes.
- Python sync contracts: 15/15 passed, including original pinned Git baseline comparison, runtime/overlay equality, exact reconstruction, idempotency and upstream drift rejection.
- An upstream v0.2.20 candidate was reconstructed with no unexpected paths; its actual Gemini regression suite passed.
- First GitHub CI at `299a692fe` passed Linux full unit tests, integration tests, Gemini race tests, synchronization contracts and frontend checks; lint findings were corrected in the follow-up commit. The final GitHub run is the release gate.
- Local full Windows tests exposed missing `sh` for three PgDumper tests (passed after Git usr/bin was added to PATH) and the existing `TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort` timestamp-sensitive failure. The corresponding Linux full suite passed in GitHub CI.
- No paid model calls were used for implementation verification. OpenCode/WorkBuddy live end-to-end validation still requires the built candidate endpoint.

## Operational notes

- Owned source/tests are mirrored under `.laffey/overlay`; upstream edits are exact transforms. A context mismatch intentionally blocks future snapshot generation.
- The resource budget bounds cumulative serialized input, not exact retained heap. The 4/16/64 KiB fragment benchmark allocated approximately 151/602/2342 KiB, approximately proportional to input.
- Late signature updates share an in-request identity record with collected parts; processing is synchronous. Patch B must define immutable snapshots before asynchronous cache use.
- Build/publish is performed by GitHub Actions Release, not the user's local machine. Production deployment is separate from this change.
