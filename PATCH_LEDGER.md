# Rareities/rclone patch ledger

This ledger is required by the CloudBridge + rclone master handoff. WP00 established the
source baseline; WP01 found no safe targeted engine source change to add. Do not infer patch
provenance from the old Round Sync handoff or from closed pull requests.

| Purpose | Upstream issue/PR | Rareities commit | Upstream equivalent | Removal condition | Tests |
|---|---|---|---|---|---|
| WP01 bounded no-change decision | N/A: no reproducible defect requiring a targeted patch | N/A: local `19d72e8` is documentation only; source baseline is `b9ba7bb` | None selected; Rareities head `1583cce…` is the exact merge base of upstream candidate `1e925200…` | Replace only after review of an exact immutable candidate and successful standalone/native compatibility gates | `go build -buildvcs=false -mod=readonly ./...`; focused sync/operations and Proton/Internxt tests — PASS |

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
