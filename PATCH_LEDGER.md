# Rareities/rclone patch ledger

This ledger is required by the CloudBridge + rclone master handoff. WP00 established the
source baseline. The earlier WP01 no-change decision below is preserved as history only and
was superseded after the exact upstream candidate and standalone engine gaps were reviewed.
Do not infer patch provenance from the old Round Sync handoff or from closed pull requests.

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
