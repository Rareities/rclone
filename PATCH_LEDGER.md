# Rareities/rclone patch ledger

## Current cross-repository verification checkpoint — 2026-09-27 (no new engine source)

- The current CloudBridge pass did not alter Rareities/rclone source or its immutable app
  pin. The fork branch remains `codex/luna-engine-final` at
  `6533d01df0a6395833d6910938e2d1fa97628ea7d`; CloudBridge continues to build against
  exact source `cf3ad40d29d15919af116a5d1e64e0381e2ce3fd`.
- Existing Go 1.26.8 Windows/amd64 evidence remains valid for the unchanged branch:
  focused `backend/mega`, `cmd/bisync`, `fs/sync`, `fs/operations`, `fs/accounting` and
  `fs/cache` tests pass, and `go vet -mod=readonly ./...` passes. The full short suite is
  **NOT PASS** because the checkout lacks the upstream `fstest/testserver/init.d` scripts
  required by FTP, HDFS, SFTP, SIA, SMB, Swift and WebDAV integration cases; the WebDAV range
  case also fails because its harness server ignored the requested range. These unavailable /
  harness-limited cases are not converted to skips or claimed as acceptance.
- The go-mega candidate remains a supporting, non-promoted dependency; race/live-MEGA,
  cross-platform, Proton/provider, Samsung/device, hosted-CI, signing and release gates are
  **NOT RUN/NO-GO**. No upstream/original PR is created or updated; the user's PR hold remains
  in force until all coding, documentation and verification work is complete.

## Windows test portability correction — 2026-09-27 (PRs deferred by instruction)

- Test-only commit `dc80d83` makes the independent Windows suite portable without changing
  production transfer or SSH behavior: config password-command tests use an argv-safe Windows
  shell, logger scripts use in-process deterministic line/tree comparison instead of Unix-only
  `sort`/`diff`, and SFTP external-session tests use platform-appropriate shell commands.
- Focused checks pass with Go 1.26.8: `fs/config`, `fs/logger`, and the two SFTP external-session
  regressions. The final complete `go test -short -mod=readonly -count=1 ./...` run from this
  commit also passes the core Bisync/sync/operations/cache/accounting scopes and WebDAV range
  behavior; it exits 1 because the host lacks launchable
  `fstest/testserver/init.d` fixtures for FTP, HDFS, SFTP, SIA, SMB, Swift and WebDAV, and the
  current WebDAV range harness reports that its server ignored the requested range. These are
  explicit unavailable/harness-limited cases, not converted to skips or claimed as passes.
- `go vet -mod=readonly ./...`, Proton/device/release gates and upstream/original PR activity
  remain separately tracked; PR creation stays deferred by user instruction.

## Final independent verification — 2026-09-27 (PRs deferred by instruction)

- Rareities/rclone `codex/luna-engine-final` is verified at
  `cf3ad40d29d15919af116a5d1e64e0381e2ce3fd` (`rclone: record safety guard and test-isolation
  fixes`). No new engine source change was needed in this pass; the branch is already pushed to
  the Rareities fork and the commit is unsigned.
- Exact isolated Rareities/go-mega commit `24d3fadc8735096fafbaac00eb3a95a2017033e5` on
  `Rareities/go-mega:codex/luna-wp01-final` passes standalone short tests/vet and the exact
  rclone `backend/mega` integration test. It is not promoted into production `go.mod` because
  race/live-MEGA, full cross-platform, and release-provenance evidence are **NOT RUN**.
- Independent core rerun with Go 1.26.8 and `-mod=readonly` passes `backend/mega`, `cmd/bisync`,
  `fs/sync`, `fs/operations`, `fs/accounting`, and `fs/cache`. Full `go vet -mod=readonly ./...`
  passes.
- The prior complete `go test -short -mod=readonly ./...` run exited 1. Its Unix-helper and
  WebDAV-range observations were repaired at the test layer by the correction above; the final
  run still exits 1 only for the unavailable Windows test-server init scripts listed above.
- WP07/WP09 remain partial: core Bisync/root-guard slices pass, but worker drain, Proton
  auth/refresh ownership, official-client interoperability and live disposable Proton tests are
  open or **NOT RUN**. Samsung/device, race, multi-process, and release acceptance remain
  **NOT RUN/NO-GO**. No upstream/original PR is created or updated.

## Latest WP00 test-isolation evidence — 2026-09-27

- WP09 root safety is now fixed at the Proton backend owning layer: `Purge` and `Rmdir` reject sanitized empty/dot/slash root spellings and focused tests verify that ordinary subdirectory lookup remains reachable. `go test -short -mod=readonly ./backend/protondrive -count=1` and `go vet -mod=readonly ./backend/protondrive` pass offline with no credentials/network. This does not close Proton: upload-worker drain, cached-login error classification, refresh ownership, hash/cache invalidation, official-client interoperability, race and live disposable-vault acceptance remain open/NOT RUN.
- A read-only WP07 Bisync review made no source change: strict listing/state inspection and absolute aggregate deletion limits already fail closed and focused `cmd/bisync` tests pass offline. Do not infer that this closes WP07; app preflight/mutation integration and full engine acceptance remain open.
- CloudBridge current-source JVM suites now pass 439/439 per flavor after app-side bounded diagnostics/worker changes; this is recorded in the app ledger and is not an engine integration or release pass.
- The active dirty source now passes `go test -short -mod=readonly -count=2 ./fs/sync -args -individual` in 37.083s. The owning defect was test-global configuration leakage: transform tests used `context.Background()` while setting name transforms, so later tests inherited `prefix=tac/tic`. All transform-test contexts now use `fs.AddConfig`; this is test-only. The existing nested-directory mtime fixture pin and accounting reset are retained. Do not treat a non-isolated shared-harness repeat as a package failure or as acceptance evidence.
- WP09 audit remains **PARTIAL** despite offline `backend/protondrive` tests/vet and 100-repeat focused auth/cache checks. Root `Purge`/`Rmdir` guards, Proton upload worker drain semantics, typed cached-login fallback, shared-config refresh ownership, nil-link handling, pacer use and official-client/live disposable Proton acceptance remain unresolved or **NOT RUN**. No dependency promotion or app pin change is authorized.
- WP01 go-mega remains a local candidate only. Latest candidate hashes are `mega.go` `1D35C246775C49AAC40B3C0903A695093CD2B2683534599E7D7F5E2FEEE89F95` and `context_test.go` `240AC8CF96EAA08DC05BFAE43AAA0C1D7578810A9A0426F7EE574F6096E9A4A6`; independent publication/provenance and race/live-MEGA evidence are still missing.

## Current verified correction — WP00/WP01 — 2026-09-27

- Active dirty source `fs/sync` passes once with `-short -mod=readonly -count=1`; `TestRcCopy` and `TestCopy` pass separately at count 2, and `TestCopyMetadata` alone passes at count 2. Both nested no-transfer timestamp tests pass separately at count 100 after a test-only fixture pin. The whole `fs/sync` package at count 2 failed again (108.727s) across many RC/copy/sync/metadata tests, including `TestRcCopy`, `TestRcMove`, `TestRcSync`, `TestCopy`, `TestCopyMetadata`, `TestSyncIgnoreErrors`, and `TestNothingToTransferWithEmptyDirs`. Root cause remains unresolved; independent read-only diagnosis is active.
- A direct Windows temp-volume probe round-trips 100ns file/directory times. The stale source listing snapshot theory is plausible, not confirmed; the old “coarse filesystem precision” characterization is superseded. No production sync change was selected.
- The isolated go-mega candidate after node-generation/trash-move repairs has hashes `mega.go` `4D75189031BC56021D7A2022CC88BAB328A21A600F1949A79048B77AE15FDCDF` and `context_test.go` `3F0E5316FC43F45E08A65E14363ED7C8C6313D72DB7C48EAED8425D12EEBA2AB`. Standalone full tests count 20, vet, gofmt, six-package local rclone integration (`fs/operations`, `fs/sync`, `fs/accounting`, `fs/cache`, `backend/mega`, `cmd/bisync`), and scoped vet passed. Independent review cleared the node-generation/trash/API/upload repairs but found one P2: a `Download` created before session replacement can issue chunk/finalization work with its old node. Dalton is repairing and testing it. Race/live MEGA checks remain **NOT RUN**; WP01 stays unaccepted.

## Historical continuation update — superseded by the current verified WP00/WP01 correction above

- Repeated Bisync state/backup tests now assign unique `accounting.WithStatsGroup` scopes. The focused backup-root reuse regression and full `go test -short -mod=readonly -count=2 ./cmd/bisync` pass on the dirty development source. This change is limited to test isolation; it does not alter engine behavior. One diagnostic run with an overlong `GOTMPDIR` failed on Windows path syntax and is excluded from test evidence.
- The broader six-package `-count=2` run failed in `fs/sync`; the independent reproduction reports roughly 1ms directory-modtime difference against a reported 100ns local precision. The exact target failed once in ten repetitions on the dirty checkout and passed once on the clean app pin. It remains an intermittent Windows baseline finding; no production fix is selected without a source-level root cause.
- Candidate go-mega standalone tests (`-count=20`), vet, formatting, and rclone `backend/mega`/`cmd/bisync` integration pass after the current session-install serialization patch. A fresh independent review found two P2s still open: synchronization of ordinary credential-dependent operations with session replacement, and clearing stale old-account filesystem/share-key cache on replacement. Candidate changes are being addressed in the isolated go-mega workspace; no dependency pin or PR update has occurred.
- Existing upstream PR #64 remains unchanged at `f8790e439632960cfe8fb6e2d3ac3ecc73122720`; refreshed GitHub data showed it open/mergeable with no reviews, inline threads, combined checks, or PR-triggered workflow run. WP01 is not accepted.

This ledger is required by the CloudBridge + rclone master handoff. WP00 established the
source baseline. The earlier WP01 no-change decision below is preserved as history only and
was superseded after the exact upstream candidate and standalone engine gaps were reviewed.
Do not infer patch provenance from the old Round Sync handoff or from closed pull requests.

## Current review snapshot (2026-09-27)

- GitHub refs were refreshed: `Rareities/rclone` master and canonical `rclone/rclone` master are both `9dc8b71ae99496460f07373674609571918bfb9c`. GitHub reports no open PRs or recent Actions runs for the fork; combined commit statuses are empty. The archived `oss-singularity/rclone` parent endpoint returns 404. The official beta site is a build distribution generated from master, not a distinct Git repository. No remote ref was changed.
- CloudBridge still pins the clean, immutable rclone source `fe775a8b58cf217fdf4bd34f0975af1e4c19c1a0`. Its four-ABI native build and current OSS debug APK passed; no engine pin change is included in this snapshot.
- Independent clean-checkout tests on the app pin (`fe775a8...`) passed `fs/operations`, `fs/accounting`, `fs/cache`, `backend/mega`, and `cmd/bisync`; the first `fs/sync` run failed `TestNothingToTransferWithEmptyDirs` because Windows measured a 502.3µs modification-time difference against a 100ns test window. A standalone rerun of the test and one full `fs/sync` rerun both passed; the full suite with only that test skipped also passed, and scoped `go vet` passed. Preserve the first failure as an intermittent baseline finding pending repeated stable runs; do not count the skipped test as pass evidence.
- Active development remains dirty on `codex/luna-engine` at `a881c6ac9bfd9509ce8ca8a93cdb51997ee45fac`, with 90 changed paths (79 generated docs report modified status but no content diff). Preserve all current edits. GitHub comparison shows the app pin diverges from current upstream by 26 ahead/6 behind, merge base `cfb90e3ebed479119718e3ae44b1171b060079e9`; active branch is at that base plus 25 local commits and 6 upstream commits still to merge. Do not fast-forward or cherry-pick blindly.
- The local go-mega test candidate was integrated using a test-only workspace. The earlier cancellation/cache-commit and poller lock-order P2s have been fixed; a later independent review cleared lock ordering but found a new P2 where malformed late FS data can leave a partial cache snapshot. That regression fix is in progress. Production rclone/app pins remain unchanged.
- Before the new partial-snapshot finding, the candidate (`mega.go` SHA-256 `271B4127561617CC714B7E4709CEB896C8CDC6AEB1A10A93C6425759D05681CF`; `context_test.go` SHA-256 `729825D6F131384BE0ED100C612953B467316841726BF496AFEA5A73AD7D6C37`) passed its full suite 20 consecutive times, `go vet`, and formatting checks. Re-testing the active rclone workspace against it passed `backend/mega` and `cmd/bisync` plus scoped `go vet`. No upstream PR update or production dependency pin has occurred. Race testing is unavailable on this Windows host (`CGO_ENABLED=0`, no GCC toolchain), so it is **NOT RUN**. These results are pre-final and do not accept WP01.
- The partial-refresh issue is now fixed in the candidate by preflighting the full response against an isolated `MegaFS` with the current/response share-key state before deterministic replay into the live cache. The regression checks cache/map/sequence preservation and old-poller restart; an additional successful-refresh test checks existing node-pointer reuse. Current candidate hashes at that test point: `mega.go` `0584BFBDB776FBD8F424C4397A986631ABEBB254A2477DBFCBF00B7760E000D9`; `context_test.go` `548EB8932A8A4ED6EA206CE1837BBDA934D035BB531DE986D33E60A4F5575706`. Full suite `-count=20`, vet, gofmt and rclone `backend/mega`/`cmd/bisync` integration plus vet passed. Plato's next review found a separate P2: session reinitialization (`sid`/master key) is not serialized with refresh. A follow-up is underway; rerun integration after it. No PR or production pin update yet.

## Historical review snapshot (2026-09-25)

- Rareities/rclone remote `master` is observed at `1583cce1e28340e5d064ed955179f5f2b31e7757`.
  CloudBridge pins the same repository URL at immutable app revision
  `fe775a8b58cf217fdf4bd34f0975af1e4c19c1a0`; that detached local checkout is clean.
- GitHub PR #1 for `codex/luna-engine` was closed unmerged: GitHub reported a 180-commit,
  260-file diff against the stale Rareities `master` base. The pinned head had no combined status
  checks or PR-triggered workflow runs in the 2026-09-25 refresh. Do not reopen or publish that
  broad diff without first establishing and reviewing a narrow base/scope.
- This local development ledger is on `codex/luna-engine` at
  `a881c6ac9bfd9509ce8ca8a93cdb51997ee45fac` (199 commits ahead of local `origin/master`). Its 79
  generated `docs/content/commands/` modifications are pre-existing task artifacts: do not edit,
  stage or include them in a source change. The clean exact-pin checkout is separate.
- The older WP09 warning below that any reusable-login initialization error may erase cached
  credentials is historical and is superseded by the later backend fix/regression recorded under
  the current WP09 implementation entry. Current code preserves cached credentials on the tested
  transient initialization failure path. This does not close WP09: independent Proton library
  worker/semaphore and refresh-identity tests, broader compatibility, dependency promotion and
  official-client/live disposable-vault acceptance remain open or **NOT RUN**. Keep the pinned
  dependency stack until candidates pass review.
- Current WP08 remains partial: backup-root reuse has a recorded local regression, but incomplete
  listing/backup-write failure injection, app-owned reservation/restore integration and device/
  provider acceptance remain open. No source PR is currently open and no release is implied.

| Purpose | Upstream issue/PR | Rareities commit | Upstream equivalent | Removal condition | Tests |
|---|---|---|---|---|---|
| Historical WP01 no-change decision (superseded) | N/A | N/A; based on the earlier `1e925200…` snapshot | Candidate later advanced; see current WP01 record below | Historical only | Do not use as current acceptance evidence |
| WP01 Bisync lock ownership and deletion limits | No upstream patch selected; defect fixed at standalone engine layer | `cb5805f` | Exact reviewed source base `cfb90e3…`; no equivalent combined lock/absolute-delete guard selected | Replace only with a reviewed equivalent preserving cross-process exclusion, fail-closed legacy recovery and absolute deletion bound | Bisync full package/stress tests, Windows/Linux-arm64/Android-arm64 builds — PASS; full evidence below |
| WP01 Internxt TOTP reauthentication | No dependency patch; backend-local capability preservation | `6833aa5` | Historical `thies2005/rclone` pin `94f543c…` was compared; new bounded behavior preserves clock-skew TOTP without its cooldown globals/retry loop | Remove only after capability review and regression coverage prove equivalent supported reauthentication | Full Internxt tests and 100-repeat TOTP tests — PASS; live account NOT RUN |
| WP09 per-Fs Proton auth callback ownership | N/A: reproduced backend defect; no dependency PR selected | `8b8d665243e778918184c7e9775d548903e3b95e` | Backend-local; no dependency patch | Replace only after an equivalent reviewed upstream ownership fix and retained two-map regression test | Focused and full Proton package tests, 100-repeat regression, vet — PASS; details below |
| WP08 native Bisync state inspection and preservation primitives | No upstream issue/PR selected; the safety gap is in native listing/recovery inspection | Local `a2eec9f`; tree-equivalent published commit recorded in the root execution ledger | No reviewed upstream read-only state-inspection API selected | Remove only after an upstream API is reviewed for bounded parsing, lock ownership, non-migrating legacy detection, and equivalent Windows behavior | Full `./cmd/bisync` package, vet, Android/arm64 build, CLI smoke, exact-byte backup/restore fixtures — PASS; app initialization/recovery/device/provider acceptance remains open |

The refreshed Rareities/rclone master identity on 2026-09-23 is
`1583cce1e28340e5d064ed955179f5f2b31e7757`. Before adding a patch, record the root cause,
owning layer, exact local commit, dependency provenance, tests, and whether the change remains
useful when rclone is used independently.

The earlier candidate `1e92520076ccc319fdae29e6fdc6a75bd523b5a2` and its 170-commit
comparison are historical. The reviewed immutable upstream candidate is
`cfb90e3ebed479119718e3ae44b1171b060079e9`; the refreshed comparison reported 174 upstream
commits ahead of the Rareities base (240 files; 9,443 insertions and 2,223 deletions). GitHub's
commit API reports the upstream tip signature as `verified: true`, `reason: valid`; Rareities'
base commit is reported unsigned. The candidate was merged into the history-preserving local
branch as merge commit `5987de7861f35d8258a757d875ca4d9615f4cbcd`. The reviewed change categories
include failed-empty-transfer Bisync accounting (`7e17e1b`), immutable copy/move semantics
(`dc98c21`, `575633c`), RC authentication and numeric bounds (`939f82d`, `7723091`, `976d05e`),
confined directory/archive paths and Zip Slip fixes (`935197b`, `842430d`), redirect-header
credential protections (`399bc6a`, `aef94cd`, `31a8164`), serve startup/cancellation fixes
(`9b9fd3f`, `f2a390b`, `b82a024`, `caa3418`), Internxt lookup/upload fixes (`77ec281`), and
security dependency updates (`ef69687`, `f550317`). The merge includes upstream dependency
updates in `go.mod`/`go.sum`; the local WP01 patches add no new dependency. The refreshed GitHub
API showed zero open PRs and zero master workflow runs for both Rareities repositories. This is
local review work, not a push or permission to rewrite the remote branch.

## Historical WP01 decision record — superseded 2026-09-23

The Rareities source was independently checked with the existing Go 1.26.8 toolchain and
existing workspace caches. `go build -buildvcs=false -mod=readonly ./...` passed, as did
`go test -mod=readonly ./fs/sync ./fs/operations` and
`go test -mod=readonly ./backend/protondrive ./backend/internxt` (the test commands returned
cached PASS results). No source files changed and no custom patch was introduced.

At that point, because the upstream difference was an unreviewed 170-commit fast-forward, the archive checkout
does not contain full upstream history for a reviewable local fast-forward, and no targeted
defect or fork-only patch was identified, WP01 records a bounded no-change decision. The
current engine remains independently useful and validated; the upstream candidate is deferred
until its exact changes can be reviewed and tested. CloudBridge's existing app pin was not
changed in this package.

## WP09 bounded slice — Proton auth callback ownership

**Finding:** Proton auth and deauth callbacks were package-level functions that looked up
`_mapper` and `_saltedKeyPass` globals when invoked. `NewFs` overwrote the mapper for each
filesystem, and config reads/writes also overwrote the salted key password. The pinned
`go-proton-api v1.0.4` retains callbacks on each client and invokes them later during token
refresh or definitive deauthentication (`client.go`, `authRefresh`). This makes callback
ownership outlive the global values that originally identified its filesystem.

**Evidence and failure mode:** Added
`TestProtonAuthHandlersStayBoundToTheirConfigMap`, using only in-memory synthetic credentials
and two config maps. Before the fix, after constructing the second filesystem, invoking the
first client's auth callback left the first map unchanged and wrote its refreshed UID/tokens
into the second map; the first client's deauth callback then cleared the second map. The
pre-fix focused test failed on these assertions. The Bridge v1.0.5 login path stores the
callbacks with the Proton client, and the pinned SDK calls them on refresh/deauth, so the
ordering is reachable without a live Proton account.

**Severity:** High for credential integrity and multi-remote isolation: one remote's session
refresh/revocation could corrupt or erase another remote's saved session.

**Scope/owner:** `backend/protondrive` (rclone backend); no Android workaround or dependency
fork is appropriate. The defect reproduces even when rclone is used as a standalone CLI.

**Action and changed files:** Replaced the process-global callback state with a mutex-protected
`protonAuthState` instance created per `newProtonDrive` call. Its method callbacks retain that
filesystem's config mapper and salted key password across refresh/deauth, and the initial-login
and cached-login fallback paths update/clear the same state. Removed `_mapper` and
`_saltedKeyPass`. Changed `backend/protondrive/protondrive.go` and added the in-memory regression
test in `backend/protondrive/protondrive_internal_test.go`.

**Dependency provenance/removal:** No dependency or `go.mod`/`go.sum` changes. Existing pinned
stack remains Proton-API-Bridge v1.0.5, go-proton-api v1.0.4, and gopenpgp/v3 v3.4.1. Remove
this backend-local patch only if a reviewed upstream backend version provides equivalent
per-filesystem callback ownership and the retained two-map test still passes.

**Validation:** The pre-fix test failed as expected. After the fix, all passed with Go 1.26.8
and `-mod=readonly`: `go test -mod=readonly ./backend/protondrive -run
'^TestProtonAuthHandlersStayBoundToTheirConfigMap$' -count=1`, the full
`go test -mod=readonly ./backend/protondrive -count=1`, the focused regression repeated with
`-count=100`, `go vet -mod=readonly ./backend/protondrive`, and `git diff --check`. Tests used
an absent disposable `RCLONE_CONFIG` path; no config was created and no Proton credentials or
network were used. Race-detector testing was not available in this environment (`CGO_ENABLED=0`
and no GCC/Clang toolchain detected).

**Limitations and remaining WP09 findings:** Proton login/refresh with the official client,
revision interoperability, cloud mutations, and live disposable-vault acceptance remain NOT
RUN. This slice does not address worker/permit cleanup, hash/cache invalidation, or SDK refresh
races. A separate source-level risk remains: `newProtonDrive` clears cached credentials on any
reusable-login initialization error before password fallback, including errors that may be
transient; do not treat this as fixed by callback isolation.

**Next WP09 step:** Add a local injected-error regression around reusable-credential
initialization: transient network/server failures must preserve the saved refresh token and
salted key password, while a specifically classified invalid/revoked credential may take a
bounded recovery path. Implement only after that behavior is reproducibly tested. Continue
independently with Proton library upload-worker/semaphore cancellation and failed-block tests;
official-client and disposable-vault checks remain prerequisites for claiming Proton acceptance.

## Current WP01 implementation record — 2026-09-23

### Package record (13-field format)

1. **Objective:** refresh and independently validate Rareities/rclone, preserving useful
   standalone behavior and closing reproduced engine safety/capability gaps.
2. **Scope:** reviewed upstream `cfb90e3…` merge; Bisync process-lock ownership and aggregate
   deletion guard; Internxt TOTP reauthentication; source/help documentation and regression tests.
3. **Out of scope:** app integration beyond updating its immutable pin after this gate; live
   Proton/Internxt account acceptance; S26/device testing; unrelated backend rewrites.
4. **Preconditions:** WP00 archive identities and exact GitHub refs refreshed; Go 1.26.8
   available; local `codex/luna-engine` descends from the Rareities history, not the archive root.
5. **Design:** merge the exact reviewed upstream commit without moving remote refs. Bisync uses
   a persistent companion OS lock and owner-token metadata; heartbeat age never grants takeover.
   Add a positive aggregate deletion ceiling of 25 by default, retaining the per-path percentage
   check at 10%; `--force` bypasses only the percentage check. Preserve Internxt TOTP capability
   with current/adjacent 30-second windows and retry only explicit 401/403 rejections.
6. **Safety invariants:** no lock file/guard unlink race; OS ownership is authoritative; unreadable,
   legacy or unsupported metadata fails closed; stale heartbeat cannot steal a live lock; guard
   is checked after complete listings and before mutation; zero/negative explicit limits reject;
   secrets/codes are not logged; no generic auth retry or cooldown loop.
7. **Implementation:** merge commit `5987de7`; Bisync commit `cb5805f`; Internxt commit `6833aa5`.
   Lock metadata v2 is atomically published/replaced while `.lck.guard` remains persistent.
   Dry-run keeps prior no-lock/no-artifact semantics. RC and CLI expose validated
   `maxDeleteCount`/`--max-delete-count`; user documentation describes recovery and version
   transition behavior. Internxt `totp_secret` is Sensitive/IsPassword and supports obscured
   values plus legacy Base32 plaintext.
8. **Existing code reused:** `gofrs/flock` already in `go.mod`; rclone config obscuring,
   SDK `HTTPError` status classification, existing delta accounting and Bisync test harness.
9. **Code retired:** time-based lock expiry as authority; direct removal/recreation of `.lck`;
   the prior 50% default; no supported Internxt automatic TOTP refresh when switching away from
   the historical custom engine.
10. **Failure behaviour:** active lock conflict returns before sync; stale/legacy metadata is
    not auto-deleted; failed owner verification/release is reported; excessive observed deletes
    abort before propagation even with `--force`; invalid TOTP seed or rate-limit/network/server
    error stops without another login request.
11. **Tests:** Go 1.26.8, `-mod=readonly`. Full `./cmd/bisync` and `./backend/internxt` tests
    PASS; Bisync lock tests previously passed at `-count=20`; TOTP cases passed at
    `-count=100`. `./fs/operations`, `./fs/accounting`, `./backend/protondrive`, `./fs/cache`,
    and `./backend/cache` PASS. Cache/VFS-backed suites used a unique `LOCALAPPDATA` under task
    temp; no existing AppData cache was accessed. Upstream review suites PASS for
    `./backend/archive/...`, `./backend/s3`, `./fs/march`, `./fs/fshttp`, `./lib/rest`,
    `./cmd/serve/http`, `./cmd/serve/s3`, `./cmd/serve/sftp`, `./fs/rc`, and `./fs/config`.
    `./fs/config` required the bundled Git `usr/bin/echo.exe` on PATH because these Windows
    tests invoke a POSIX-style `echo` executable; the corrected run passed. `./cmd/rc` and
    `./cmd/serve/ftp` have no test files. The first `./cmd/serve/s3` attempt used the protected
    default AppData cache and failed on access; the full package passed after cache redirection.
    `./backend/local` passes when skipping exactly `TestSymlinkRangeBeyondEnd` and
    `TestDirBTimeThroughPlantedSymlinkBlocked`; both unfiltered failures were reproduced on the
    pristine exact-upstream checkout because this account lacks Windows symlink privilege.
    `./fs/sync` passes when skipping exactly `TestNothingToTransferWithEmptyDirs` and
    `TestNothingToTransferWithoutEmptyDirs`; those unfiltered failures were reproduced on
    pristine `cfb90e3` due Windows directory mtime precision. One initial test invocation named
    nonexistent `./fs/config/rc`; the corrected `./fs/config` package test passed. Changed-package
    vet PASS; full `go vet ./...` reports only the same baseline iCloud unreachable-code and
    Linkbox mutex-copy findings. Full `go build -buildvcs=false -mod=readonly ./...` and
    Windows/amd64, Linux/arm64 and Android/arm64 builds PASS. CLI help displays the new limits;
    explicit zero rejects before filesystem setup. Race detector NOT RUN (`CGO_ENABLED=0`; no C
    compiler). Local patches add no dependency; reviewed upstream `cfb90e3` updates `go.mod`/
    `go.sum`. Generated website command/backend docs were not committed per `AGENTS.md`; package
    RC docs and the hand-maintained Bisync page were updated.
12. **Acceptance status:** standalone implementation/test milestone PASS with explicit
    environment limitations and two independently reproduced upstream sync-test failures.
    GitHub reports the exact upstream tip signature as valid; local patch commits are not
    signed. No CI runs are evidenced. Live Proton and Samsung Galaxy S26 / One UI 8.5/9 acceptance remain
    NOT RUN. CloudBridge still requires its post-WP01 immutable pin update and native APK
    inspection before WP07 integration proceeds.
13. **Rollback point:** revert only `cb5805f` and/or `6833aa5` as bounded local patch commits;
    retain upstream merge `5987de7` and WP09 `8b8d665` unless separately re-reviewed. Keep the
    app on prior pin `1583cce…` until it is deliberately updated to a verified final WP01
    commit; do not delete existing Bisync profiles or metadata.

The archive-derived checkout remains separate rollback evidence and must not be pushed as
upstream history. No push or PR has yet been made. This standalone evidence is not Android-native,
live Proton, device, or release acceptance.

## WP08 partial native-state and preservation slice — 2026-09-23

**Finding/evidence:** Native `bilib.BasePath` can rename legacy suffixed listing files, while the
existing listing loader tolerates malformed rows. Calling either as a preview/probe could mutate
state or misclassify corrupt/interrupted metadata as a usable baseline. A Windows-specific
fixture caught a further path bug: `filepath.Join(base, ".path1.lst")` looked for a directory
named after the base rather than the native `base.path1.lst` file. The regression now asserts
legacy state is reported unknown and left byte-for-byte in place.

**Failure mode/severity:** Treating missing, partial, malformed, stale-owner, or migrated legacy
listings as a fresh profile could lead a later resync to overwrite or delete the losing version.
This is a high-severity data-preservation risk.

**Scope/owner and independent value:** `cmd/bisync` owns native listing format, lock ownership,
and backup semantics. The new inspector and tests remain useful to rclone CLI users without
CloudBridge; this is not an Android workaround.

**Action:** Added versioned `InspectState` JSON statuses (`ABSENT`, `COMPATIBLE`, `INTERRUPTED`,
`INCOMPATIBLE`, `UNKNOWN`), a bounded strict listing parser, native OS guard acquisition,
lock-metadata checks, legacy-name detection without rename, and old-listing validation without
restore. Added `--inspect-state`, rejecting explicitly mutating combinations. The docs clarify
that `ABSENT` applies only to the supplied workdir and provider construction may authenticate or
make metadata requests. Tests cover active owners, malformed/type-divergent listings, valid
recovery evidence without mutation, absent workdirs, one/both-empty initialization, file/directory
conflicts, unusable backup targets, same-size/same-mtime losing bytes, and exact byte restore to a
separate disposable target.

**Dependency/provenance:** No new Go dependency. Reused the existing `gofrs/flock` dependency and
native Bisync comparison/listing structures. Local source commit is `a2eec9f`; its parent tree
matches the published branch tree exactly. The published tree-equivalent commit and pinned app
revision are to be recorded after the branch update is verified.

**Validation:** On Windows with Go 1.26.8 and `-mod=readonly`, the focused WP08 safety set passed;
full `go test -json -mod=readonly ./cmd/bisync -count=1` passed (42.013 s);
`go vet -mod=readonly ./cmd/bisync` passed; `GOOS=android GOARCH=arm64 go build -mod=readonly
./cmd/bisync` passed. CLI smoke returned the expected bounded `ABSENT` JSON for a unique missing
workdir and rejected `--inspect-state --resync`. Only disposable local directories and an absent
config path were used. No Proton credentials/network were used. No device, race-detector, or
process-kill stress acceptance is implied.

**Status, remaining findings, and order:** This is a partial WP08 native foundation, not WP08
acceptance. App preview, explicit initialization/recovery UI and coordinator, durable
run-scoped backup locations, recovery-evidence persistence, backup permission/disk/network fault
injection, cancellation/kill-boundary tests, migration rollback, full native-integrated Gradle
build, Galaxy S26/One UI and live Proton verification remain open or NOT RUN. Do not enable the
legacy Bisync execution path; a missing established profile workdir stays UNKNOWN. Next, publish
and verify the immutable engine revision, pin CloudBridge to it, and finish app-side preview/init/
recovery before broad integration or release decisions.

**Rollback:** Revert only local commit `a2eec9f` to remove this inspector/CLI extension. Retain
the preexisting native lock/deletion protections. The CloudBridge pin stays on its previous
verified SHA until the new immutable branch commit is fetched and its tree/native artifact are
verified. Never delete or auto-prune existing listings, backups, profiles, or recovery artifacts.

## WP08 publication and app-pin verification — 2026-09-23

The WP08 source and ledger were published to `Rareities/rclone` branch `codex/luna-engine` by a
non-force fast-forward from `ec863fdcd9e1ce0d13357f791d5528aab10bf0ec` to
`d53551e1722305268c6072263f11066f1278a4a0`. Published tree
`98c402c28fda112c56b543b3104841435fb8cdf6` exactly matches the locally tested tree (source
commit `a2eec9f17e9624a8ed78afeccf520c5716270b1f` plus the WP08 documentation ledger commit).
CloudBridge now pins that immutable commit; profile fixtures were updated to match.

This ref refresh does not imply release provenance: GitHub reports the API-created commit as
unsigned, no PR-triggered CI run is evidenced, and neither default branch was changed. The app's
prior full `:rclone:buildAll` integration run fetched this exact SHA and built
`armeabi-v7a`, `arm64-v8a`, `x86`, and `x86_64`; the later app-only v13 migration verification
used `-x :rclone:buildAll` because the engine source/pin was unchanged. Native output hashes and
test details remain in the app execution ledger. No APK or release artifact is implied.

Standalone WP08 acceptance remains partial: full `./cmd/bisync` tests, package vet and
Android/arm64 cross-build passed; process-kill/race stress, actual Android instrumentation,
Galaxy S26 / One UI 8.5/9 and live Proton acceptance are **NOT RUN**. Keep existing Bisync
listings/backups and profiles untouched; the CLI inspector never restores or initializes them.

## WP09 — Proton backend hardening and dependency-candidate audit — 2026-09-23

**Local implementation:** commit `becac91d51f48013df0fdc87da78d7d6ffafa6ed` updates only
`backend/protondrive/protondrive.go` and its internal tests. Name-hash lookup now falls back to
decrypted directory names only on a clean hash miss, propagates listing errors, filters file vs
folder kinds, and treats a matching entry without link metadata as an error. Move and DirMove
invalidate both source and destination subtrees using each filesystem's own cache/path encoder.
Reusable-login construction errors no longer erase saved credentials before password fallback;
definitive SDK deauthentication still clears credentials through the per-Fs callback.

**Local evidence:** `go test -mod=readonly ./backend/protondrive -count=1`,
`go vet -mod=readonly ./backend/protondrive`, `gofmt -d` for both changed files, and
`git diff --check` passed on the committed source. `go test -mod=readonly
./backend/internxt ./backend/drime -count=1` also passed, preserving non-Proton backend coverage.
Tests use synthetic maps/entries and no Proton credentials or cloud data. Race-detector testing
is **NOT RUN** (`CGO_ENABLED=0`; no GCC/Clang toolchain is installed).

**Reviewed upstream candidates (refreshed 2026-09-23):**

- rclone/rclone#9851 hash fallback (`868b105143a9b3b1f0fefc41c6d151f17d5b1c2b`) and #9813
  destination-cache invalidation (`e5328dabf194f5fc4cfc02ab27d1f4d3b5b19b61`) remain open.
  The backend-local implementation is adapted with stricter malformed-entry behavior and
  source/destination `Fs` ownership; current Rareities `go.mod` stays unchanged.
- Proton-API-Bridge#8 (`47d69aac6987f1c91d454b0d61edc8267d6a83e5`) remains open. Its exact
  isolated checkout passed `go test -mod=readonly ./...` and `go vet -mod=readonly ./...`.
  Race tests are **NOT RUN** for the toolchain reason above. No CI status checks were attached
  to the commit in the refreshed GitHub API response; it is not pinned in production.
- go-proton-api#10 (`9dcca00ba1dc779a8958c9d4988b79ce2f9f3233`) and
  Proton-API-Bridge#9 (`6f7fc6530d4a77d33d08e36464e018600ae4b93d`) remain open. API10 root tests,
  the focused refresh-hook tests, and vet passed. Its full `go test ./...` failed in the
  large `server` test package after 139.986s with Windows socket-bind/connect errors;
  the previously failing label-test group passed when rerun alone. Bridge9 full tests and vet
  passed in a temporary `go.work` paired with the API10 checkout. No PR CI status checks were
  attached to either exact head.
- A test-only regression added to the isolated API10 checkout proved that its current hook will
  adopt a *different UID* from a replaced config and can return the other account's user data.
  A temporary local guard requiring the persisted UID to equal the live client UID (and requiring
  nonempty UID/access/refresh tokens) made all focused hook tests pass. This audit patch is not in
  Rareities/rclone or an upstream commit; do not adopt API10 until identity binding is reviewed,
  upstreamed, and versioned.
- go-proton-api#5 (`cdb5bd158f14d8b763035b13cf950e34e416ec89`) and
  Proton-API-Bridge#4 (`b0b05e7b89f9605ccb6af1f4682000f3275fea3f`) provide newer SDK base/move
  and revision compatibility, but both refreshed PRs report `mergeable: false`. API5's focused
  v2 route tests and vet passed. Bridge4's focused tests plus vet passed except
  `TestSetMoveLinkSignatureAddressCompat`, which fails because the helper leaves
  `SignatureAddress` empty while its test expects it to be populated. No live SDK/Proton move or
  revision acceptance was run. This stack is not pinned.
- Bridge#3 (`9ba9207356aef0eb85acb31285bf28feeb4ffd84`) includes deleting a draft link when
  revision listing fails; rejected because an unavailable list cannot authorize deletion.
  Bridge#6 (`b75484eb1509c571c10d2c67547d12366638e729`) skips signature verification; rejected.
  Bridge#7 (`a4a88cd59199faa88a25181ef164d8779499bb03`) removes a 5-second delay without a
  bounded consistency proof; defer pending read-after-move evidence.

**Dependency/pin decision:** Rareities/rclone remains on Proton-API-Bridge v1.0.5,
go-proton-api v1.0.4, and gopenpgp/v3 v3.4.1. No production `replace`, unpublished local
dependency, unmerged PR head, or unverified SDK behavior was introduced. `NewFs` currently
constructs separate `ProtonDrive` instances; no package-global Fs coalescing lock exists, so no
global lock/deadlock risk was added. The optional bandwidth/browser-login work remains out of
this bounded slice.

**Status:** WP09 remains **PARTIAL**. Hash lookup/cache invalidation and transient credential
preservation are implemented at the rclone backend owner and independently tested. Worker
cleanup, safe refresh preservation, and current-SDK revision/move interoperability remain gated
on upstream fixes that pass identity/safety review and become reproducibly pinnable. Live
Proton, disposable-vault mutations, official-client revision round-trips, and Samsung acceptance
are **NOT RUN**. Release readiness is not claimed.

**Rollback:** revert only local commit `becac91`; retain per-Fs callback ownership from
`8b8d665`. Do not roll dependencies forward to any reviewed PR head until its source, tests,
version/provenance, and app integration are verified. Never delete or recreate remote objects to
work around revision failures.

## WP08 successful dry-run state follow-up — 2026-09-23

**Source commit:** `175c3508193eb13be1b5d8c39b8b26475bf4c58c`
(`bisync: preserve accepted state across dry runs`).

**Finding:** after a successful native `--dry-run`, rclone retains `.lst-dry*` scratch outputs
while preserving the canonical `.lst` files. The WP08 inspector initially classified those
non-authoritative scratch outputs as unresolved interrupted state, so merely previewing a
compatible profile would block its next preflight. Treating these files as recovery listings
would also be wrong: they are not the accepted baseline and must never authorize recovery.

**Change and safety:** `InspectState` now ignores only `.lst-dry*` artifacts when classifying
durable state. The active native guard, stale active lock metadata, `.lst-new`, `.lst-err`, unsafe
files, partial/missing accepted listings, and malformed/divergent listing checks remain
fail-closed. No dry-run output or accepted listing is deleted. Tests exercise a dry-run initial
resync (roots remain unchanged and state still reports `ABSENT`) and a compatible-state dry-run
with same-size/same-mtime/different-byte content (both endpoint bytes and each canonical listing
remain exact; inspection remains `COMPATIBLE`). The checksum comparison is explicit so that the
test actually observes the same-metadata content difference.

**Validation:** Windows Go 1.26.8: `go test ./cmd/bisync -count=1` PASS (40.573 s),
`go vet ./cmd/bisync` PASS, and `GOOS=android GOARCH=arm64 go build -mod=readonly ./cmd/bisync`
PASS. `gofmt` and `git diff --check` PASS. No dependency changed; no Proton/network data was used.

**Status and remaining work:** this is a native WP08 follow-up only, not WP08 acceptance. The
source commit is local and has not been published; CloudBridge still pins `d53551e…` until a
refreshed safe publication and tree/build verification. App preview UI/worker, durable backups,
restore/recovery, failure/kill injection, integration against the new SHA, Galaxy S26 / One UI
8.5/9 and Proton disposable-area checks remain open or **NOT RUN**. No APK or release is implied.

**Rollback:** revert only `175c350`; retain the previous read-only inspector and all existing
Bisync state/listings/backups. Do not prune dry-run or recovery artifacts as a workaround.

## WP08 path-free native dry-run summary — 2026-09-23

Source commit: `12ef1fd7e200fd47444c4c7044d23856d80fda04`
(`bisync: expose path-free dry-run preview summary`).

1. **Objective:** provide CloudBridge a stable, bounded native summary for an explicitly requested
   Bisync dry-run without parsing human logs or treating the summary as execution authorization.
2. **Scope:** CLI-only `--preview-json`; versioned result fields for status, planned transfer/byte/
   file-delete/directory-delete counts, error count and explicit unknown conflict count; stats getter;
   CLI/RC and Bisync docs.
3. **Out of scope:** app preview persistence/UI/worker, initialization/recovery, backup/restore,
   exact conflict enumeration, provider/device acceptance, Proton changes, or release claims.
4. **Preconditions:** prior native inspector/dry-run state work (`a2eec9f`, `175c350`); no
   dependency change. CloudBridge continues to pin the previously published immutable engine ref
   until this source is safely published and integration-tested.
5. **Design:** require `--dry-run` and reject `--inspect-state` before filesystem construction;
   emit exactly the versioned JSON summary after a successful Bisync return. Mark native errors or
   retry/fatal conditions `INCOMPLETE`; never serialize endpoint paths, object names, or error
   prose. Set `conflictsKnown:false` rather than implying native conflict enumeration.
6. **Safety invariants:** preview output does not authorize a later mutation and is not a snapshot;
   failed runs cannot emit a successful summary; inspection remains a distinct read-only operation;
   no test data or provider state persists after the disposable CLI smoke.
7. **Implementation:** commit `12ef1fd` adds `cmd/bisync/preview.go` and tests, validates flag
   combinations before `cmd.NewFsSrcDstFiles`, writes the summary only after `Bisync` succeeds,
   adds `StatsInfo.GetDeletedDirs`, and documents the CLI-only protocol.
8. **Reuse:** existing native Bisync dry-run and accounting stats; no replacement for native
   comparison, deletion guards, or accepted-state inspection.
9. **Retired:** no prior behavior retired; the summary avoids a future app dependency on parsing
   path-bearing human output.
10. **Failure behavior:** invalid flag combinations return an error before backend construction;
    native execution errors return normally without a JSON success object; a completed dry run
    with native accounting/retry errors is labeled `INCOMPLETE`.
11. **Tests:** Windows Go 1.26.8: `go test -mod=readonly ./cmd/bisync ./fs/accounting -count=1`
    PASS (37.649s and 1.394s); `go vet -mod=readonly ./cmd/bisync ./fs/accounting` PASS;
    `go build -buildvcs=false -mod=readonly ./...` PASS; `GOOS=android GOARCH=arm64 go build
    -buildvcs=false -mod=readonly ./cmd/bisync` PASS; `gofmt` and targeted `git diff --check`
    PASS. A disposable local CLI smoke produced one stdout JSON object with `COMPLETE`, one
    planned transfer and 26 bytes; without `--dry-run`, the command returned nonzero and rejected
    the option. Both temporary roots/config/workdir were removed after exact path validation.
12. **Acceptance:** PARTIAL native preview protocol only. Commit is local, not yet published or
    pinned by CloudBridge. App-owned fresh preview/known-unknown persistence, cancellation,
    process-death behavior, backup/restore/fault boundaries and init/recovery remain open.
    Proton disposable tests and Galaxy S26 / One UI 8.5/9 acceptance remain **NOT RUN**. No APK,
    signing, PR/CI or release gate is implied.
13. **Rollback:** revert only `12ef1fd`; retain the previous Bisync command, native inspector,
    deletion protections and accepted listings. Never prune state or treat dry-run artifacts as
    a recovery baseline.

## WP08 downstream CloudBridge pin and four-ABI integration — 2026-09-24

This follow-up supersedes the earlier note that CloudBridge had not pinned the native summary
protocol; it does not complete the rclone or app work package.

1. **Objective:** verify the published path-free native preview protocol is consumable by the
   Rareities/CloudBridge Android build at an immutable revision.
2. **Scope:** downstream pin at the app repository, native ABI compilation and debug APK
   packaging; rclone production source is unchanged by this entry.
3. **Out of scope:** app preview worker/UI/persistence, safe initialization/recovery, live Proton,
   device acceptance, release signing, or GitHub PR/CI completion.
4. **Preconditions:** Rareities/rclone `codex/luna-engine` remote SHA
   `fe775a8b58cf217fdf4bd34f0975af1e4c19c1a0`, tree
   `68df728c1eb3cd1e60340286cd99548e6d0d0452`; independent Go validation for WP08 preview is
   recorded above.
5. **Design:** consume `https://github.com/Rareities/rclone.git` by full commit SHA, with no
   floating branch or upstream fallback.
6. **Safety invariants:** the app build fetched the exact source SHA and version; generated native
   objects remain build outputs; debug signature is not a release signature; no remote user data
   was touched.
7. **Implementation:** CloudBridge commit `eb71b94fb563d75ce02c161530fe9ec594038642` updates
   its ref/identity fixtures and Go executable resolution. `:rclone:buildAll` freshly compiled
   all four ABIs; OSS debug assembly packaged all four libraries.
8. **Reuse:** rclone `--dry-run --preview-json`, prior native state inspector and existing
   Gradle `rCloneRepoUrl`/`rCloneRef` integration.
9. **Retired:** downstream pin `d53551e…` is superseded by `fe775a8…`. The older `d53551e…`
   commit remains in ancestry and may be restored only with an explicit rollback.
10. **Failure behavior:** missing engine source/ref continues to fail Gradle configuration; no
    app summary is considered known until its future parser validates the exact protocol.
11. **Tests:** fresh Windows Go 1.26.8 `:rclone:buildAll` PASS (arm64-v8a, armeabi-v7a, x86,
    x86_64). CloudBridge JDK 17.0.20.1 / Gradle 8.13 unit tests PASS (14 suites, 76 tests,
    0 failures/errors, one symlink skip), lint PASS with baseline, instrumentation-source compile
    PASS, `assembleOssDebug` PASS. Universal APK SHA-256
    `71F6EA499419D1928E50BD2F211126E1E4BD6952B3B5552E981FF095626614CE`, verified debug signer
    SHA-256 `2dfd7565c808a6144fd1c7458b5dfe5fc04b31319f23055c8e4d0d80096273bb`. The rclone
    commit is unsigned; no source-signature claim is made.
12. **Acceptance:** downstream engine pin and four-ABI debug integration PASS; app-level preview
    work, native initialization/recovery, backups/fault tests, R8/release, PR/CI, actual Android
    instrumentation, Galaxy S26 and live Proton tests are open or **NOT RUN**. No release
    readiness is claimed.
13. **Rollback:** revert the app pin and its engine-identity fixtures together to the previous
    immutable `d53551e…`; retain native state/listings/backups and do not downgrade user state.

## WP08 local Bisync backup-root reuse regression on active engine worktree (PARTIAL)

1. **Objective:** establish whether reusing a native Bisync backup root can overwrite an older
   preserved version at the same relative path, motivating a fresh run-scoped root invariant.
2. **Scope:** test-only change in `cmd/bisync/backup_safety_test.go`, active local
   `codex/luna-engine` worktree; source/test commit `67a7b94`. No production behavior changed.
3. **Out of scope:** native partial-backup failure semantics, remote/provider behavior, app
   manifest or restore flow, engine pin changes, PR/CI, APK, signing, or release acceptance.
4. **Preconditions/provenance:** local pre-change HEAD was `4fd2ba3d5df2650dd49177f0690dc80f767c0b9b`.
   GitHub confirms the public `codex/luna-engine` ref is exactly `fe775a8b58cf217fdf4bd34f0975af1e4c19c1a0`,
   the CloudBridge pin. Fetched blob IDs for selected Bisync state/operations files, ProtonDrive
   source, and `go.mod` match local `4fd2ba3…`, but GitHub cannot resolve that local commit and
   full tree/ancestry equality remains unverified. Do not publish this worktree or attribute this
   test to the public pin without reconciling history.
5. **Design:** for both Path1 and Path2, create disposable local endpoint/backup/work roots,
   establish an accepted baseline, pre-populate the selected backup root with older bytes, change
   the opposite endpoint using a same-length version, run native Bisync with both backup flags,
   then assert endpoint convergence, unchanged sentinels, and replacement of the older backup.
6. **Safety invariants:** all test paths are beneath `t.TempDir`; no real provider, user data,
   CloudBridge `.android/`, accepted profile, or non-disposable backup is accessed. Existing
   generated command docs remain unstaged and untouched.
7. **Implementation:** commit `67a7b94` adds one table-driven test covering both backup roots;
   follow-up `4d245f1` adds the repository-standard Go copyright header. No dependency or
   production file changed.
8. **Existing code reused:** Bisync native backup flags, local backend, `stateInspectionTestFs`,
   existing `mustRead` helper, and the package's local disposable test conventions.
9. **Code retired/decision:** no code retired. Treat native backup-root reuse as unsafe for an
   app retry or uncertain run; each mutation attempt needs fresh unique backup locations.
10. **Failure behavior:** this test demonstrates replacement on a successful reuse; it does not
    establish behavior when backup transfer partially writes and then fails. App mutation remains
    unavailable until its durable run-owned preservation and restore contract is proven.
11. **Tests:** `git diff --cached --check` passed before commit; the independent static review
    found no obvious API/compile mismatch. `go test`, compilation, gofmt, and runtime execution
    are **NOT RUN** because no Go toolchain is on PATH in this checkout. No CI evidence exists.
12. **Acceptance status:** regression source committed locally; this test is not verified by
    execution, WP08 remains **PARTIAL**, and no claim is made for the published pin or package
    acceptance. Public branch comparison is 180 commits ahead of `master`; no open rclone PR,
    workflow runs, or status checks were returned for the pinned commit.
13. **Rollback/next:** if review or execution finds a defect, revert `4d245f1` and then
    `67a7b94`; do not alter the published ref or unrelated generated docs. Next, run this test in
    Go-enabled CI and add an injected partial-backup-write failure test for both sides before
    considering the broader WP08 preservation gate.

## 2026-09-25 - WP08 backup-root reuse regression execution revalidation (PARTIAL)

1. **Objective:** execute the previously source-only native backup-root reuse regression and
   revalidate its owning Bisync/ProtonDrive packages with the available pinned Go toolchain.
2. **Scope:** no source changes; validated active `codex/luna-engine` checkout at
   `894298219d4f3798b5507fee5f457464bab6547c`, including test source commit `67a7b943818d38a97b3aced02a3a7c962f719292`.
3. **Out of scope:** publishing the branch, changing CloudBridge's pin, introducing production
   backup behavior, provider/Proton interoperability, app restore flow, PR, APK, signing, or
   release.
4. **Preconditions:** Go 1.26.8 Windows/amd64 from the task-local toolchain; read-only module
   mode; `GOPROXY=off`; isolated absent config; all test-created endpoints and backup roots are
   disposable local paths.
5. **Design:** run the Path1/Path2 backup-root replacement regression repeatedly, then the
   complete affected package suites, static checks, full repository build, and Android/arm64
   Bisync cross-build.
6. **Safety invariants:** test data is confined to `t.TempDir()` under a short task-owned mapped
   temp root; no user/provider data, credentials, app `.android/`, or generated command docs were
   changed. The 79 pre-existing generated command-doc modifications remain unstaged and untouched.
7. **Implementation:** no files changed. Toolchain/cache/temp configuration was scoped to the
   task workspace. The short mapped temp drive was removed after the suite completed.
8. **Reuse:** existing `TestBisyncBackupDirReuseReplacesExistingPreservedPath`, native Bisync
   local backend, existing test fixtures, and the repository's existing module cache.
9. **Retired/decision:** the earlier `NOT RUN` result remains correct for that prior environment
   but is superseded by this execution record. No source or upstream behavior was changed.
10. **Failure behavior:** the first broad suite attempt with the long workspace temp path failed
    on Windows generated-session-name/path limits in existing tests. Re-running with the short
    disposable temp path passed; no test was suppressed or altered.
11. **Tests:** focused backup-root reuse test passed 20/20; `go test -mod=readonly -count=1
    ./cmd/bisync ./backend/protondrive` passed (`cmd/bisync` 27.343s, ProtonDrive 0.541s);
    `go vet -mod=readonly ./cmd/bisync ./backend/protondrive`, `go build -buildvcs=false
    -mod=readonly ./...`, `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build
    -buildvcs=false -mod=readonly ./cmd/bisync`, `gofmt -d cmd/bisync/backup_safety_test.go`,
    and targeted `git diff --check` passed. Go module downloads were disabled. Race detector,
    full live Proton interoperability, Samsung device, and Linux/Android runtime tests are
    **NOT RUN**.
12. **Acceptance:** the regression and affected native packages are independently verified on
    this local checkout. This is not evidence for the CloudBridge-pinned `fe775a8…` revision or
    the separate `codex/luna-preview-isolation` checkout; WP08 remains **PARTIAL**, and no
    native follow-up is published or app-integrated.
13. **Rollback/next:** no code rollback applies. Reconcile candidate changes against a clean
    Rareities/rclone base and review them before any PR/pin promotion; continue WP08 endpoint
    placement/manifest/restore proof. Keep app mutation, live-provider claims, and release gates
    closed.

## 2026-09-25 - WP08 opt-in RCD route-deny candidate (AUTHORED, NOT EXECUTED)

1. **Objective:** provide a native RCD boundary that can refuse Bisync route dispatch before a job is created, complementing CloudBridge's argv-mode fence.
2. **Scope:** uncommitted candidate edits in `fs/rc/rc.go`, `fs/rc/rcserver/rcserver.go`, and `fs/rc/rcserver/rcserver_test.go` on `codex/luna-engine`; no app pin update.
3. **Out of scope:** enablement by CloudBridge, command-graph integration assumptions, all mutation safety semantics, provider/app acceptance, publication, APK, or release.
4. **Preconditions:** WP07/WP08 ingress review identified that an RCD caller could dispatch `sync/bisync` independently of the app's process argv fence.
5. **Design:** opt-in repeated `rc_deny_commands` exact-route list, snapshotted when the server is constructed; authorization happens first, then a denied route returns 403 before job creation.
6. **Safety invariants:** default remains allow-compatible; unauthenticated requests retain 401 behavior; exact route matching does not deny similarly prefixed commands; no rejected request starts a job.
7. **Implementation:** server option/map and pre-job check added; tests use the existing `rc/list` command without mutating the process-global command registry and cover empty-default, exact-vs-sibling route, authenticated deny and unauthenticated precedence.
8. **Reuse:** existing RC options, route canonicalization/dispatch, auth middleware and HTTP test helpers.
9. **Retired/decision:** rejected a test design that registered fake global routes because registration state could leak across tests. Do not pass `rc_deny_commands` from CloudBridge until this candidate is built/tested and the integrated binary's Bisync route registration is verified.
10. **Failure behavior:** option is opt-in and exact; the app's existing native Bisync fail-closed fence remains authoritative until tested engine integration exists.
11. **Tests:** `git diff --check` passed. Go compilation/tests and `gofmt` are **NOT RUN** because this checkout lacks Go and scoped official-toolchain downloads failed at proxy authentication; no download/bypass or CI evidence exists.
12. **Acceptance:** source-only WP08 candidate; not compiled, executed, committed, published, pinned, or app-integrated. WP08 remains **PARTIAL / BLOCKING**.
13. **Rollback/next:** discard/revert only these three scoped candidate hunks if Go tests or review reveal a problem; obtain a Go-enabled execution environment, run focused `fs/rc` and `fs/rc/rcserver` tests plus formatting, inspect build command registration, then review before any immutable pin promotion. Keep all Bisync mutation gates closed.

## 2026-09-25 - WP08 nested job/batch deny-policy follow-up (AUTHORED, NOT EXECUTED)

1. **Objective:** close the reviewer-confirmed bypass that let `job/batch` invoke an RCD command omitted by the outer HTTP path's deny check.
2. **Scope:** request-context deny snapshot in `fs/rc/jobs/job.go`, propagation from authenticated `fs/rc/rcserver/rcserver.go` dispatch, and an authenticated nested-batch regression test.
3. **Out of scope:** CloudBridge integration, enabling the option in the app, changing default rclone behavior, broad route policy, Bisync mutation semantics, CI or publication.
4. **Preconditions:** independent static review found `job/batch` reaches `NewJobFromParams` directly, bypassing the HTTP handler's outer route check.
5. **Design:** copy the immutable server deny set into the authenticated request context; apply exact-path denial inside `NewJobFromParams` before `jobs.NewJob` for nested dispatches.
6. **Safety invariants:** missing auth still rejects before request execution; empty configuration remains allow-compatible; exact paths only are denied; blocked nested commands allocate no child job or invoke the handler.
7. **Implementation:** added `jobs.WithDenyCommands`, server request-context propagation, and a `job/batch` test denying nested `rc/list` with status 403 in the nested result.
8. **Reuse:** existing route canonicalization, context propagation, `NewJobFromParams`, RC error payload and `job/batch` test harness.
9. **Retired/decision:** removed the assumption that guarding only the URL path protects all indirect dispatch; retained direct-route auth-before-deny ordering and default allow.
10. **Failure behavior:** denied nested paths return an RC error with status 403; other nested jobs are unchanged. Context without a deny policy remains behavior-compatible.
11. **Tests:** static reviewer finding confirmed by code-path inspection and implementation/test authored. `gofmt`/Go tests remain **NOT RUN** because Go is unavailable; the official Go 1.26.8 archive attempt failed proxy authentication. No toolchain bypass, commit, app pin update or CI run occurred.
12. **Acceptance:** **WP08 PARTIAL / NOT ACCEPTED.** Candidate source and regression test remain uncompiled/unexecuted; the app remains pinned to `fe775a8b58cf217fdf4bd34f0975af1e4c19c1a0` and does not enable the option.
13. **Rollback/next:** run `gofmt` and focused `go test ./fs/rc/jobs ./fs/rc/rcserver ./fs/rc ./cmd/bisync` in an authorized Go-enabled environment; rerun review for direct, nested, unauthenticated and default-allow paths before any scoped commit or app integration.

## 2026-09-25 - WP08 nested deny follow-up independent review (NOT EXECUTED)

- A second, independent static review found no bypass in direct route denial or nested `job/batch` dispatch: auth remains first, the server deny set is snapshotted, request context propagates it to serial/concurrent nested paths, and `NewJobFromParams` checks exact paths before child job creation.
- The review changed no files and ran no tests. Go compilation, `gofmt`, focused package tests, build-command registration audit, CI, commit, app pin promotion, and app integration remain **NOT RUN**. This review does not change the WP08 partial/blocking status.

## 2026-09-25 - WP08 RCD deny-policy implementation and independent package tests (PARTIAL)

1. **Objective:** close the authenticated direct-RCD and nested-batch Bisync ingress while keeping ordinary rclone behavior unchanged by default.
2. **Scope:** `fs/rc/rc.go`, `fs/rc/jobs/job.go`, `fs/rc/rcserver/rcserver.go`, focused `fs/rc/rcserver/rcserver_test.go` coverage, and the global docs/content/flags.md entry.
3. **Out of scope:** app option enablement, direct CLI policy, mutation safety outside this RCD route, WP07 acceptance, provider/device testing, and publication.
4. **Preconditions:** WP08 requires WP07; the standalone engine candidate can be independently validated, but this slice is not application-level WP08 acceptance.
5. **Design:** snapshot exact deny paths on server creation, reject authenticated direct calls before job creation, and propagate the same policy through serial/concurrent `job/batch` dispatch.
6. **Safety invariants:** empty defaults allow; auth precedes denial; rejected commands allocate no jobs; invalid noncanonical paths fail server startup instead of silently disabling the intended guard.
7. **Implementation:** added repeatable `rc_deny_commands` and documented `--rc-deny-commands` in the global flags list; added immutable request-context propagation to `NewJobFromParams`, a direct exact-path check, and canonical path validation rejecting empty paths, whitespace/controls, leading/trailing/repeated slashes, backslashes, query/fragment delimiters, dot segments, malformed escapes and percent-encoded aliases. Tests use a copied, restored RC registry so the synthetic `sync/bisync` handler cannot pollute later test state; its invocation count remains zero for direct and batch denials. The registry-swapping test is explicitly sequential.
8. **Reuse:** existing option registry, HTTP authentication, RC error response, job/batch dispatch and route matching.
9. **Review/decision:** an independent review found that percent-encoded command paths could be accepted as configuration but not match decoded request paths; startup now rejects escapes and the regression covers encoded slash variants, invalid escapes, query and fragment delimiters. Isolated owner-layer tests cover `sync/bisync`, `rc/list`, missing credentials, exact matching, default-empty policy, malformed configuration, serial batch and concurrent batch. A full `cmd/bisync` integration test remains unavailable offline due uncached cloud modules; actual command registration in the integrated native binary is not established.
10. **Failure behavior:** malformed deny config causes startup error; a denied direct or nested registered command returns 403 before job/handler invocation. The option remains opt-in.
11. **Tests:** Go 1.26.8 Windows/amd64, offline: `go test -count=25 -run '^TestDenyCommands|^TestDenyCommandSet|^TestNewServerRejectsMalformedDenyCommand' ./fs/rc/rcserver` **PASS**; `go test -shuffle=on -count=5 ./fs/rc/rcserver` **PASS**; `go test -count=1 ./fs/rc/...` **PASS**; `go vet ./fs/rc/...` **PASS**; `git diff --check` exit 0 (CRLF normalization warnings only). `go test -race` is **NOT RUN** because `CGO_ENABLED=0` and no GCC is installed. `cmd/bisync` integration is **NOT RUN** because dependencies are unavailable with `GOPROXY=off`.
12. **Acceptance:** **WP08 PARTIAL / NOT ACCEPTED.** WP07 and its WP01/WP04-WP06 preconditions remain unaccepted; provider-backed preservation, exact restore, restart reconciliation, mandatory mutation authorization and app integration are open. CloudBridge stays pinned to `fe775a8b58cf217fdf4bd34f0975af1e4c19c1a0`.
13. **Rollback/next:** retain only after review of the exact candidate diff. Do not commit/push/pin against the current 199-commit-ahead local branch until its GitHub base and dirty generated-doc set are reconciled. Continue the master package order and keep Bisync init/apply/recovery unavailable.

## 2026-09-25 - WP08 RCD dispatcher bypass and startup-side-effect closure (PARTIAL)

1. **Objective:** prevent alternate HTTP/job dispatch from reaching a denied command and avoid changing process-global job defaults when RCD server configuration fails.
2. **Scope:** `fs/rc/rc.go`, `fs/rc/rcserver/rcserver.go`, `fs/rc/rcserver/rcserver_test.go`, `docs/content/flags.md`, and this ledger.
3. **Out of scope:** CloudBridge integration, production pin promotion, mutation authorization beyond RCD, provider/device acceptance, publication, APK or release.
4. **Preconditions:** independent review confirmed that `core/command` can launch arbitrary rclone CLI commands as a child process and bypass a route-only deny of `sync/bisync`.
5. **Design:** reject the generic CLI dispatcher whenever any exact RC deny policy is active; create/validate the server before assigning global job defaults.
6. **Safety invariants:** auth is checked before denial; direct and nested denied calls are rejected before job/handler execution; empty policy preserves `core/command`; malformed configuration and failed server construction do not change job defaults.
7. **Implementation:** `Server.isCommandDenied` now denies `core/command` for any non-empty policy, documents this conservative behavior in the option help and flags text, and retains exact route matching. `Start` builds the server before `jobs.SetOpt`. The synthetic Bisync test label now explicitly states that it uses a synthetic registry handler, not the production `cmd/bisync` registration.
8. **Reuse:** existing auth-before-dispatch flow, `jobs.WithDenyCommands`, `NewJobFromParams`, RC status payloads and test harness.
9. **Review/decision:** an independent reviewer found the `core/command` bypass; after closure, a second review found no remaining HTTP/job dispatch bypass in scope. A built root executable's `rc/list` confirmed the actual `sync/bisync` registration and a loopback RCD request verified its denial.
10. **Failure behavior:** blocked direct/generic-dispatch calls return HTTP 403 before job allocation; nested serial/concurrent batch calls retain the deny context; defaults remain unchanged when a server cannot be constructed.
11. **Tests:** Go 1.26.8 Windows/amd64: focused direct/nested/generic-dispatch/path/startup tests repeated 25 times **PASS**; shuffled rcserver five runs **PASS**; all `./fs/rc/...` tests and `go vet ./fs/rc/...` **PASS**; root `go build -buildvcs=false` **PASS**; full `go test -count=1 ./cmd/bisync` **PASS** (39.039s using a short workspace temp root); all `TestLockfile*` top-level tests repeated 100 times **PASS** (180.898s); focused concurrency and aggregate-delete tests repeated 10 times **PASS**. Loopback on the assembled binary confirmed the production route is listed, direct Bisync and `core/command` return 403, and missing credentials return 401. Initial Bisync suite on a long system temp path failed two tests; targeted and full reruns using a short task temp root passed. Scoped `git diff --check` **PASS** with line-ending normalization warnings only. Race testing is **NOT RUN** (CGO disabled; no GCC).
12. **Acceptance:** engine-side RCD/build integration is verified on this Windows candidate, but WP08 remains **PARTIAL / NOT ACCEPTED** because WP07 and WP01/WP04-WP06 prerequisites and provider/app acceptance remain open. No app pin change or Android integration occurred.
13. **Rollback/next:** keep the deny policy opt-in; do not commit, publish or promote the pin until branch/base reconciliation and built-binary tests. Preserve Bisync init/apply/recovery as unavailable.

## 2026-09-25 - WP01 local-only history classification (READ-ONLY)

1. **Objective:** distinguish upstream history, branch-specific code and audit-only commits before deciding whether this local engine history is promotable.
2. **Scope:** `codex/luna-engine` HEAD `a881c6ac9bfd9509ce8ca8a93cdb51997ee45fac`, local `origin/master` `1583cce1e28340e5d064ed955179f5f2b31e7757`, and the current-upstream comparison recorded in the umbrella ledger.
3. **Out of scope:** fetch, reset, rebase, cherry-pick, commit, push, branch cleanup, app pin changes, or code edits.
4. **Preconditions:** fresh remote comparison reports upstream `ce5351c52e94ac0df65e4781f65f55291bae4f80` 193 commits ahead of the fork master; the local branch's raw `origin/master..HEAD` count is 199, not 199 downstream code changes.
5. **Design:** classify the 25 branch-specific commits after excluding 174 commits already represented in current upstream history; separate ledger checkpoints, mechanical tests, substantive changes, and merge ancestry.
6. **Safety invariants:** preserve the audit trail and branch state; treat local-only changes as candidates requiring source/test review; do not infer current behavior from old ledger claims alone.
7. **Implementation/result:** no code changed. Fifteen commits are ledger-only checkpoints: WP01 `7c16245c065643732eea5956ac57db9394d5f9ce`, `9c5ba4cbb2c69dc2a6bf3086fc6cd1369a19d725`, `d1cb681643aef1c8f8f17d28da3ba81fc0cefd49`, `0be5f004e14eebf486fefa76b493f573431c5375`, `ccd9f64d07bbbfb4d4eb702f91c9e82d0ed824b7`; WP08 `f35230df8c43408291beb21d7e33b65d3007ed23`, `6212da63c8de8c4721cb56428e0d6fcd5e56055e`, `2c88dcca1c534a411dc43b3ba68c7862e652e84d`, `4fd2ba3d5df2650dd49177f0690dc80f767c0b9b`, `88faa6ba7132df6b3530b0a3821c69169696655e`, `a44347bf1e25be2cfc9a723fb9ee3cadf03994ca`, `894298219d4f3798b5507fee5f457464bab6547c`, `a881c6ac9bfd9509ce8ca8a93cdb51997ee45fac`; WP09 `a57ab6e600838c2332156bfcca303d4e109b1cc0`, `980dff2a599d75283947673e09ff7732348ebf10`. Mechanical/test-only commits: `4d245f101f56156ccdd58eb4fdfabfe93ccbf325` adds a standard test header; `67a7b943818d38a97b3aced02a3a7c962f719292` adds a useful backup-root reuse regression, not a production fix. No generated command-doc-only commit was identified; dirty generated docs in the working tree are outside this commit classification.
8. **Reuse:** existing lock/process tests, preview/state tests, Proton account/session tests, Internxt TOTP implementation and patch ledger.
9. **Review/decision:** retain for deeper review: `cb5805f05038da6380d30d4287796d896565226a` adds OS-backed profile lock ownership and a 25-file aggregate deletion limit; process tests were recorded at 20 repetitions but handoff critical-race evidence targets 100. `12ef1fd7e200fd47444c4c7044d23856d80fda04` adds a path-free preview summary and `175c3508193eb13be1b5d8c39b8b26475bf4c58c` preserves accepted state across dry runs; conflict results remain unknown by design. `a2eec9f17e9624a8ed78afeccf520c5716270b1f` adds state inspection but creates a persistent `.lck.guard` in an existing workdir; resolve/document whether this internal serialization sidecar fits the read-only contract, and test an existing empty workdir. Do not unlink the guard casually: Bisync deliberately preserves it because unlinking a locked file can split lock ownership across inodes. `becac91d51f48013df0fdc87da78d7d6ffafa6ed` improves Proton lookup/cache but falls back from any reusable-login initialization error to username/password login; classify rejected/expired credentials separately and do not start another login on arbitrary timeout/5xx/429. `8b8d665243e778918184c7e9775d548903e3b95e` isolates Proton callbacks per filesystem; add concurrent refresh and token-rotation persistence tests. `6833aa59517c2680c2889a9804df21c820d357eb` adds bounded Internxt TOTP reauthentication; verify 403 means OTP rejection and keep redaction tests. `5987de7861f35d8258a757d875ca4d9615f4cbcd` is a merge/import ancestry point, not an independent production fix.
10. **Failure behavior:** transient Proton errors must not silently trigger another password login; ambiguous read-only/guard semantics must be documented and tested; lock/preview failures remain fail-closed.
11. **Tests:** this classification ran no tests and changed no files. Existing ledger evidence includes 20-repeat lock coverage and selected source tests, which do not meet WP01/WP08 race/provider acceptance. Samsung, live Proton and live Internxt remain **NOT RUN**.
12. **Acceptance:** WP01 and WP08 remain **PARTIAL / NOT ACCEPTED**. The 25 branch-specific commits are understood at a summary level, but the safety-critical code, upstream delta, compatibility/build matrix and clean publishable base still require review.
13. **Rollback/next:** preserve history. Before WP09 Proton changes, fix the fallback classifier and add concurrency/refresh preservation tests; before WP08 read-only state acceptance, settle the guard-sidecar contract. Continue selective upstream VFS/Internxt/security review and do not promote the app pin or publish this history.

## 2026-09-25 - WP08 Bisync state-inspection guard contract (PARTIAL)

1. **Objective:** resolve whether read-only native state inspection may leave its process-serialization artifact in an existing workdir.
2. **Scope:** `cmd/bisync/state_test.go`, the `--inspect-state` CLI help in `cmd/bisync/cmd.go`, and the hand-maintained `docs/content/bisync.md` synopsis.
3. **Out of scope:** changing the native lock algorithm, deleting a guard file, reading/writing endpoint objects, recovery or listing migration, CloudBridge enablement, and WP08 acceptance.
4. **Preconditions:** master WP08 is gated on WP07 and Proton prerequisites; the prior branch review had explicitly left guard-sidecar behavior to settle. The existing source already takes the native guard to avoid observing in-progress listing changes.
5. **Design:** define “read-only” as no listing/lock-metadata migration or endpoint mutation; inspection of an existing workdir may create/leave the empty OS-lock guard needed to serialize with native Bisync. An absent workdir still remains absent.
6. **Safety invariants:** do not unlink the guard after unlock: another process could hold the old inode while a third creates and locks a new inode, defeating mutual exclusion. Do not create listing or lock metadata during inspection.
7. **Implementation:** CLI help now warns that an empty `.lck.guard` may remain. A new empty-existing-workdir test asserts `ABSENT/LISTINGS_ABSENT` and that the only entry created is the regular `<session>.lck.guard` file.
8. **Reuse:** existing `InspectState` locking, state result contract, missing-workdir test, and Bisync listing-preservation tests.
9. **Review/decision:** independent read-only review confirms the persistent guard is necessary for stable inode mutual exclusion and recommends disclosing its persistence. The test additionally proves the guard remains zero bytes. Review also identified that a same-directory attacker can race the pre-open `Lstat`; docs now require a writable, trusted/private workdir and limit the guarantee to cooperating native Bisync writers. This does not harden the lock against a malicious local writer or establish arbitrary network/cloud filesystem semantics.
10. **Failure behavior:** lock acquisition errors continue to report `UNKNOWN`; no state/listing file is created or rewritten by the new test path.
11. **Tests:** Go 1.26.8 Windows/amd64 with workspace-local build/module caches and short temp directory: four focused inspection tests **PASS** (2.147s); full `go test -count=1 ./cmd/bisync` **PASS** (39.679s); after strengthening the guard-size assertion, the focused inspection rerun **PASS** (1.426s); `gofmt -d` is empty and scoped `git diff --check` exits 0 (line-ending normalization warnings only). Independent review completed. Race/other OS, provider-backed, Samsung, Proton, app integration, and hosted CI tests are **NOT RUN**.
12. **Acceptance:** bounded native WP08 contract clarification only; WP08 remains **PARTIAL / NOT ACCEPTED** and master prerequisites remain open.
13. **Rollback/next:** if later lock design removes the persistent inode requirement, revise this test/docs together; otherwise retain the guard contract. Continue WP00/WP01 and WP02-WP07 in master dependency order; do not enable Bisync initialization/apply/recovery.

## 2026-09-25 - WP01 x/crypto security delta review (PARTIAL)

1. **Objective:** verify the SSH security dependency delta in the candidate and decide whether the newer post-fix tag should be adopted.
2. **Scope:** direct `golang.org/x/crypto` use in `backend/crypt` and SFTP client/server code; current `go.mod`/`go.sum` and candidate binary metadata.
3. **Out of scope:** app engine pin change, unrelated dependency refresh, or treating an upstream version as accepted without a tested base.
4. **Preconditions:** refreshed upstream/fork comparison remains unresolved for publication; the existing candidate already selects x/crypto v0.56.0 and requires Go 1.26.0.
5. **Design:** retain the smallest security-complete release represented in the current upstream delta, and test both the dependency’s SSH package and rclone’s crypto/SFTP consumers.
6. **Safety invariants:** no dependency downgrade; no adoption of a broader transitive refresh solely because a later tag exists; no app pin change without WP01 baseline acceptance.
7. **Implementation:** no module files changed. v0.56.0 is already present in the candidate and contains fixes for [GO-2026-6303](https://pkg.go.dev/vuln/GO-2026-6303) (fixed v0.55.0), [GO-2026-6354](https://pkg.go.dev/vuln/GO-2026-6354), and [GO-2026-6355](https://pkg.go.dev/vuln/GO-2026-6355) (both fixed v0.56.0). The official [v0.57.0 tag](https://go.googlesource.com/crypto/+/refs/tags/v0.57.0) is dated 2026-09-08, descends from the last security fix, and updates only x/* dependency requirements; defer it pending WP01's full dependency/build comparison because it expands the dependency set beyond the targeted security fix.
8. **Reuse:** current module checksums, upstream security fixes, existing SFTP/crypt consumer tests, and pre-existing root executable build evidence.
9. **Review/decision:** read-only subagent review confirmed SSH server/client paths are reachable through rclone's SFTP packages and crypt uses x/crypto primitives. Do not copy fixes manually. The choice is v0.56 for the minimum validated security delta, not a claim that later versions were ignored; revisit v0.57 during the full upstream refresh.
10. **Failure behavior:** module selection and checksums remain unchanged; a failed platform integration test is recorded as environment-limited, not passed or a source defect.
11. **Tests:** Go 1.26.8 Windows/amd64, offline: `go test -mod=readonly golang.org/x/crypto/ssh` **PASS** (5.030s); selected Windows-safe `backend/sftp` host-key/path tests **PASS** (2.272s); `./cmd/serve/sftp` **PASS** (0.868s); `./backend/crypt` **PASS** (6.943s); selected `./fs/config` crypto tests **PASS** (1.936s); `go mod verify` reports **all modules verified**; selected module query confirms v0.56.0/Go1.26.0. The broader combined command is **NOT PASSED** because Unix-only SFTP `fstest/testserver/init.d` scripts and shell `echo` are unavailable on Windows; the focused Windows-safe tests above pass, but those integration paths remain **NOT RUN**. No `go.mod`/`go.sum` diff.
12. **Acceptance:** security-version review is partial WP01 evidence; WP01/WP07 remain **PARTIAL / NOT ACCEPTED** pending full upstream history/base reconciliation, broader platform build matrix, and app integration.
13. **Rollback/next:** preserve current v0.56 selection; evaluate v0.57 and current upstream go.mod together on a clean branch, then run focused SFTP/crypt, root build, and vulnerability scan. Keep the CloudBridge pin unchanged.
