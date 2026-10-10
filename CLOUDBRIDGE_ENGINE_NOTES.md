# CloudBridge engine fork

This branch is the native engine for Rareities/CloudBridge. CloudBridge pins the
full commit SHA rather than a moving branch. Changes are intentionally kept in
the fork; the Android repository does not patch a fetched checkout.

## Implemented

- Proton authentication callbacks own their filesystem mapper and salted key.
  Stale callbacks cannot overwrite or clear a newer saved session. Only explicit
  cached-login rejection triggers credential fallback; transient and integrity
  errors retain credentials.
- Backend lookup falls back to authenticated, decrypted directory names when
  legacy name hashes miss. Ambiguous matches fail rather than choose a file.
- Plaintext sizes and SHA-1 metadata fail closed when requested but unavailable;
  opened download readers retain their real Close behavior.
- Bisync creation, expiry, renewal and deletion use a retained OS guard lock,
  exclusive lock creation, inode and owner checks, and synchronized cancellation.
- Existing Android DNS overrides and Internxt token renewal/TOTP behavior are
  preserved on the newer backend. DNS overrides accept explicit IP addresses,
  preserve UDP/TCP semantics and use no hardcoded fallback. Internxt retries are
  bounded with backoff, adjacent TOTP windows and manual-reconnect terminal state.

## Validation and limits

Focused Proton, Internxt, HTTP/DNS and bisync lock tests pass with the race
detector; go vet passes for these packages. No live provider account was used.

Cross-filesystem adoption of sibling-rotated Proton refresh tokens remains an
upstream integration gap. Session isolation and stale-write guards do not solve
that issue. The backend name fallback also does not replace the API bridge's
internal upload collision lookup. Signature verification remains enabled; no
unmerged bridge/SDK dependencies or verification bypasses were imported.

The lock protocol requires all participants to use the updated engine. Older
binaries do not participate in the OS guard. Android device cancellation, reboot,
filesystem permissions and real provider behavior still require release testing.
