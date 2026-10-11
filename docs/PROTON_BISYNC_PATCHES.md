# Proton Drive and CloudBridge patch ledger

This branch builds on Rareities/rclone `3a5758eeaa7250635f756f56c84128445d33ebe2`.
The source VERSION remains the upstream development version, not a claim of a
stable release. Existing upload retry, timestamp, crypto and security fixes are
retained. In particular, consumed upload readers still use `CallNoRetry`.

## Implemented changes

| Area | Implementation | Evidence and validation |
| --- | --- | --- |
| Credential callbacks | Per-filesystem session owner, salted-key/UID generation checks, stale callback rejection and persisted refresh adoption | New concurrent mapper, reconnect, rotation, rollback and revocation tests |
| Constructor identity | SDK credentials and callbacks share one atomic mapper snapshot | Prevents a reconnect between separate reads changing callback ownership |
| Cached login failures | Preserve saved credentials on construction failures; no inline password-login storm | Only a matching SDK deauth callback can clear the current session |
| Refresh dependencies | Exact go-proton-api PR 10 commit and bridge PR 9 snapshot | Companion fake-server refresh tests; exact source and checksum in `third_party/proton-api-bridge/UPSTREAM.md` |
| Modern filenames | Shared bridge lookup falls back to exact authenticated names, covering traversal and upload conflict lookup | Legacy/modern name, duplicate, type, active/draft and signature-error tests |
| Mixed name formats | Complete authenticated candidate scan even when a legacy hash matches | A regression first reproduced hidden duplicate plaintext names across legacy and modern hashes |
| Plaintext metadata | Consistent validated SHA1; unknown plaintext size stays unknown, not ciphertext size | Empty, unavailable, malformed and ranged-read tests |
| Download ownership | Range wrappers forward Close exactly once and retain its error | Normal, early, repeated and range-boundary Close tests |
| Upload cancellation | Failed semaphore acquisition returns without release; batches cancel and join all workers | Saturated-permit cancellation, failure-and-join and successful batch fixtures |
| Strict bisync | Opt-in `--require-checksum` rejects unavailable common hashes, blank per-file hashes, unknown sizes and damaged baseline rows | Normal listing, resync-before-write, empty-file and damaged-baseline tests |
| Exclusive state | OS guard plus exclusive creation and inode-checked cleanup/renewal | Concurrent acquisition and replacement-ownership tests; full bisync scenarios |
| Android compatibility | Retained `RCLONE_DNS_SERVERS` support with proper IPv4/IPv6 parsing and requested UDP/TCP transport | DNS parsing fixtures; no new public resolver fallback |
| Internxt compatibility | Retained optional TOTP and SDK bootstrap login; bounded reauthentication backoff; reload renewed token | Existing backend tests and retained TOTP fixtures |
| Internxt recovery | Timed, capped authentication cooldown rather than a lifetime failure latch | Recovery after seven failures and concurrent-attempt tests |

## Dependency review

The refresh interfaces come from:

- `rclone/rclone#9881`, proposal head `ec82fbee41f8842c2770611858c5530afa8a5b5b`.
- `rclone/go-proton-api#10`, `9dcca00ba1dc779a8958c9d4988b79ce2f9f3233`.
- `rclone/Proton-API-Bridge#9`, `6f7fc6530d4a77d33d08e36464e018600ae4b93d`.

The backend implementation adds identity and stale-callback checks beyond the
proposal. The API replacement is pinned to its author's exact, verified module
commit. The bridge snapshot is checked into this repository because the shared
upload-name fix is not available in a released module. It retains its MIT licence.

Bridge PR 6's signature-verification bypass is not included. This branch does not
change key trust, SRP, signature verification or encryption algorithms. Unknown
drafts remain protected by the existing default of not replacing them.

## Strict comparison contract

`--require-checksum` is opt-in and requires `--compare=size,checksum --checksum`.
It rejects options that deliberately exclude checksums or substitute download
hashing. Resync validates both complete trees before its first write. Normal
bisync already lists both trees before applying deltas; the additional checks
make unavailable metadata fatal instead of treating it as equality. An empty
file has a valid hash and is supported.

The guard file remains on disk after unlock to preserve a single OS-lock inode.
Its presence alone does not indicate an active job. The existing `.lck` metadata
file and explicit recovery semantics remain separate. No automatic resync,
unconditional lock deletion or `--force` is added.

## Validation commands

```sh
go test ./backend/protondrive ./backend/internxt ./cmd/bisync -count=1
go test github.com/rclone/Proton-API-Bridge github.com/rclone/Proton-API-Bridge/common github.com/rclone/go-proton-api -run 'TestAuthenticatedName|TestAuth.*Refresh|TestLogin.*Refresh|Test.*Block' -count=1
go vet ./backend/protondrive ./backend/internxt ./cmd/bisync
go test -race ./backend/protondrive ./cmd/bisync -run 'TestSession|TestRequireChecksum|TestLockfile' -count=1
```

The race check requires a supported C toolchain. A read-only Ubuntu CI job runs
it independently of release credentials.

## Remaining compatibility gates

Local and fake-server tests do not establish complete live Proton compatibility.
A disposable-account pilot still needs official-client round trips, refresh over
a real expiry interval, multiple accounts, account-key changes and interrupted
uploads. Shared With Me, Photos and future crypto formats are not newly supported.

The mapper transaction mutex is process-local. Cross-process token rereads help
adopt rotations but are not a complete cross-process compare-and-swap protocol.
CloudBridge therefore uses conservative transfer admission and fresh job processes.
Live interaction with a concurrently browsing RC daemon still needs validation.

No battery or network-efficiency improvement is claimed as measured. Android
device launch, permissions, platform quotas and page-size compatibility must be
tested on actual target devices before a production release.

Replace each local patch only after an exact reviewed upstream version contains
an equivalent fix and the corresponding regression fixtures pass.
