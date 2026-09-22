# Rareities/rclone patch ledger

This ledger is required by the CloudBridge + rclone master handoff. WP00 established the
source baseline; WP01 found no safe targeted engine source change to add. Do not infer patch
provenance from the old Round Sync handoff or from closed pull requests.

| Purpose | Upstream issue/PR | Rareities commit | Upstream equivalent | Removal condition | Tests |
|---|---|---|---|---|---|
| WP01 bounded no-change decision | N/A: no reproducible defect requiring a targeted patch | N/A: `7c16245` is documentation only; engine baseline is Rareities `1583cce…` | None selected; Rareities head `1583cce…` is the exact merge base of upstream candidate `1e925200…` | Replace only after review of an exact immutable candidate and successful standalone/native compatibility gates | `go build -buildvcs=false -mod=readonly ./...`; focused sync/operations and Proton/Internxt tests — PASS |
| WP09 per-Fs Proton auth callback ownership | N/A: reproduced backend defect; no dependency PR selected | `8b8d665243e778918184c7e9775d548903e3b95e` | Backend-local; no dependency patch | Replace only after an equivalent reviewed upstream ownership fix and retained two-map regression test | Focused and full Proton package tests, 100-repeat regression, vet — PASS; details below |

The current Rareities/rclone master identity is `1583cce1e28340e5d064ed955179f5f2b31e7757`.
Before adding a patch, record the root cause, owning layer, exact local commit, dependency
provenance, tests, and whether the change remains useful when rclone is used independently.

As of 2026-09-23, upstream `rclone/rclone` master is
`1e92520076ccc319fdae29e6fdc6a75bd523b5a2`, 170 commits ahead of this fork according to the
GitHub comparison, with no fork-only commits reported by that comparison. This is a review
candidate for a fast-forward, not an instruction to replace source or rewrite remote history.

## WP01 decision record

The Rareities source was independently checked with the existing Go 1.26.8 toolchain and
existing workspace caches. `go build -buildvcs=false -mod=readonly ./...` passed, as did
`go test -mod=readonly ./fs/sync ./fs/operations` and
`go test -mod=readonly ./backend/protondrive ./backend/internxt` (the test commands returned
cached PASS results). No source files changed and no custom patch was introduced.

Because the upstream difference is an unreviewed 170-commit fast-forward, the archive checkout
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

## History-preserving workspace branch

The WP01 documentation commits were replayed onto `codex/luna-engine`, descended directly
from Rareities/rclone `1583cce1e28340e5d064ed955179f5f2b31e7757`. The archive-derived
checkout remains rollback evidence; do not push its unrelated root. No Go source change, push
or PR was made by this replay. The CloudBridge pin still names the immutable Rareities head.
On this history-preserving branch, `go test ./fs/sync ./fs/operations
./backend/protondrive ./backend/internxt` passed and
`go build -buildvcs=false -mod=readonly ./...` passed with the workspace Go 1.26.8 toolchain.
These are independent engine checks, not Android-native or live Proton acceptance.
