# Contained Proton dependencies

The rclone fork contains the released source modules `github.com/rclone/go-proton-api v1.0.4` and `github.com/rclone/Proton-API-Bridge v1.0.5` under relative Go module replacements. Their original licenses remain in each directory. No moving external branch or unowned development repository supplies production code.

`PROTON_BASELINES.json` records SHA256 of every original released module file. The parent rclone commit identifies the complete applied source. Go build information exposes the replacement paths; release provenance must retain the parent commit and include these paths. These directories are nested modules, so ordinary `go test ./...` does not test them automatically. Test their import paths explicitly from the parent module to use the parent's resolved dependency graph:

```sh
go test -race ./backend/protondrive github.com/rclone/go-proton-api -run 'TestAuth|TestDrive|TestRefreshHooks|TestNameFixtures|TestEncryptMessageNonAead|TestSetEncXAttrStringNonAead|TestSetNameNonAead'
go test -race github.com/rclone/Proton-API-Bridge -run 'TestModernNameLookup|TestDraftReplacement'
go vet ./backend/protondrive github.com/rclone/go-proton-api github.com/rclone/Proton-API-Bridge
```

Never run the bridge's credentialed/destructive integration tests against production data. Controlled live Proton account/official-client interoperability tests remain required before making release compatibility claims.

## Reviewed patch lineage

- go-proton-api PR https://github.com/rclone/go-proton-api/pull/10, exact head `9dcca00ba1dc779a8958c9d4988b79ce2f9f3233`: pre-refresh and post-rejection persisted-credential hooks, with fake-server tests. Additional guards reject other session UIDs, empty credentials and previously spent refresh tokens. Rejected refreshes allow a bounded persistence window. The fork preserves the persisted tuple if adoption is still unavailable: a spent-token rejection cannot establish revocation and a competing winner may persist later. Explicit current-session initialization rejection remains separately classified; cancellation does not revoke the session.
- Bridge PR https://github.com/rclone/Proton-API-Bridge/pull/9, exact head `6f7fc6530d4a77d33d08e36464e018600ae4b93d`: hook registration in both cached and first-login paths. The backend checks salted-key/account configuration and monotonic local token generation before adoption. This retains the backend's existing stale-callback protections.
- Bridge search: legacy hash matches are checked first; authenticated exact decrypted names are checked for all eligible siblings to reject mixed-hash ambiguity, even when a legacy hash is available, and covers backend lookup plus internal active/draft upload searches. State/type restrictions and ambiguity errors remain enforced. Signature or decryption failure aborts lookup.
- Bridge draft conflict: unverified draft link/revision replacement fails closed, including legacy `ReplaceExistingDraft=true`. The API metadata provides no trustworthy ownership proof. No automatic committed-object deletion, trash emptying or revision removal is added.

No signature-verification bypass from Bridge PR #6 is included. V4/V6 auxiliary-data round trips and wrong-key/unsigned-name negative fixtures retain authenticated cryptographic behavior. The upcoming official crypto migration is not implemented by these compatibility patches and remains a release readiness dependency.

## Maintenance and removal

Review upstream releases regularly against the baseline manifest and the exact changed files. Replace contained modules with tagged upstream versions only after equivalent refresh, modern-name, negative-crypto and draft-safety tests pass. Removing replacements without those checks can restore the original token race, modern-name overwrite failure or unsafe draft deletion. Do not remove licenses, pin unreviewed PR branches, or overwrite generated module caches to supply production patches.

## Fork persistence hardening

Authentication callbacks for file-backed configuration acquire a retained, canonical
configuration sidecar lock. Each guarded comparison reads the reloaded mapper;
one storage Save atomically installs the complete UID/access/refresh/salted-key/
generation tuple. A two-process race fixture verifies a stale writer cannot split
or overwrite the winning tuple; symlink aliases use the same canonical guard.
In-memory/env-defined remotes keep mapper semantics under the process-local mutex.
Other programs and configuration editors do not honor this fork-specific guard;
application configuration admission remains required.

Persistence errors stop initialization or refreshed operations through the API and
bridge durability hook. Authorized memory retains a newly rotated token; it never
rolls back to a spent token. Failure still requires user-visible reconnect/recovery.
The client request path avoids inverted/nested authentication-hook locks during
Close. Shared-client pooling was reviewed and removed because its retirement and
ownership behavior was unsafe; app admission and fresh CLI contexts bound work.

First-login durability checks also reject concurrent configuration changes rather
than return an initialized session whose credentials could not be safely stored.
