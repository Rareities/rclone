# Rareities/rclone patch ledger

This ledger is required by the CloudBridge + rclone master handoff. It starts empty because
WP00 has not yet established any new custom patch. Do not infer patch provenance from the old
Round Sync handoff or from closed pull requests.

| Purpose | Upstream issue/PR | Rareities commit | Upstream equivalent | Removal condition | Tests |
|---|---|---|---|---|---|
| No new patch recorded at WP00 | N/A | N/A | Not assessed | N/A | N/A |

The current Rareities/rclone master identity is `1583cce1e28340e5d064ed955179f5f2b31e7757`.
Before adding a patch, record the root cause, owning layer, exact local commit, dependency
provenance, tests, and whether the change remains useful when rclone is used independently.

As of 2026-09-23, upstream `rclone/rclone` master is
`1e92520076ccc319fdae29e6fdc6a75bd523b5a2`, 170 commits ahead of this fork according to the
GitHub comparison, with no fork-only commits reported by that comparison. This is a review
candidate for a fast-forward, not an instruction to replace source or rewrite remote history.
