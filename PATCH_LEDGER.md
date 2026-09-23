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
