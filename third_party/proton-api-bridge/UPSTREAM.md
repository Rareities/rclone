# Reviewed Proton bridge snapshot

This module is copied from `jomplox/Proton-API-Bridge` commit
`6f7fc6530d4a77d33d08e36464e018600ae4b93d`, the head of upstream
`rclone/Proton-API-Bridge#9`, based on bridge 1.0.5.

Module archive checksum: `h1:ITLZ6OGn1X3lcQD1Vh3J655Vrk9igDWbk/bc059IdtA=`.
The upstream MIT licence is retained in LICENSE. Source files, common and
utility packages, and local/fake-server tests are retained. Live-account
`drive_test.go`, its helper and image fixtures are intentionally omitted from
this build snapshot; they remain available at the exact upstream commit.

Local behaviour changes cover search.go, file_upload.go and their regression tests.
Two trailing spaces in constants.go are removed without changing its values. Legacy hash
availability no longer proves that a filename is absent. The shared lookup
used by traversal and uploads falls back to exact authenticated plaintext
names, rejects ambiguity/type collisions, and propagates signature errors.
Existing key selection, signature verification, encryption and draft ownership
policies are unchanged. Unknown drafts are not automatically deleted.

The authenticated lookup checks all candidates even after a legacy-hash match,
since a modern-hash object can share that plaintext name. Additional filename
decryption avoids choosing an arbitrary object during overwrite or traversal.

Block upload batches cancel remaining work after an error, drain every worker,
and release only semaphore permits that were acquired. Cancellation while
waiting for a permit cannot over-release the semaphore or strand senders.

The root rclone module pins the companion go-proton-api refresh-hook commit.
This local module snapshot is necessary while the shared upload lookup fix is
unreleased. Remove the local replacement after a reviewed upstream release
contains both the auth hook and equivalent authenticated name lookup and the
local regression suite passes. Do not edit a downloaded Go module cache.

Validation must include the parent rclone backend tests, this module's local
crypto/search tests and common's fake-server auth-refresh tests. These do not
prove compatibility with every live Proton account or a future crypto migration.
