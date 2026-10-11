# Proton bisync performance investigation

## Report and scope

The user reports 5–10 minutes before the first reported transfer byte, then
roughly 20 minutes for 40 MB across many small uploaded files. Downloads may
also be affected. No errors were reported. File counts, effective saved flags,
and a task log are not available; the root cause of that exact device run is
not established. Transfer counters do not measure metadata/crypto work.

## Verified listing bottleneck and local fix

CloudBridge deliberately sets `--protondrive-enable-caching=false` for bisync.
With that setting, the previous backend `List` built each file object through
`GetActiveRevisionAttrs` and `getLinkKR`. Each file recursively fetched and
decrypted its ancestors again, even though `ListDirectory` had already
verified the folder key. For N siblings at depth D, this repeats approximately
N × D ancestor requests and key decryptions during a fresh scan.

`ListDirectoryWithAttrs` now resolves the verified folder key once and derives
each file key directly from it. The backend builds objects from those decrypted
attributes without another ancestor walk. Each listing still fetches current
children, checks child-parent membership, verifies names/node keys/revision
attributes, rejects ambiguous names, and fails as a whole on authentication
errors. Missing attributes retain unknown plaintext size instead of ciphertext
size. No inventory cache, persistent key cache or authentication bypass is added.

The original names-only bridge method remains available to other callers.

### Measured result

The real-PGP HTTP fixture uses 20 signed/encrypted sibling files below a root
and folder, with metadata caching disabled. No network latency is injected and
fixture/key generation is excluded from timing. Three measured iterations on
this Linux/amd64 host using Go 1.26.9:

| Path | Ancestor GETs per scan | Time per scan |
| --- | ---: | ---: |
| Previous per-file ancestor resolution | 42 | 2.047 s |
| Shared verified parent | 2 | 0.712 s |

This fixture shows 95.2% fewer ancestor requests and approximately 2.9× faster
scanning. It is not an Android or live-Proton throughput measurement and does
not establish how much of the reported 5–10 minute wait it removes.

### Validation

- Proton backend, bridge, bridge/common and bisync tests passed under `-race`.
- Tests cover fresh second listings, cancellation, concurrent scans, separate
  node/revision signers, corrupt names/node signatures/attributes, unexpected
  parents, missing revision properties and missing-attribute size fallback.
- A host CLI build succeeded with `-buildvcs=false`; its version command ran.
  Workspace module VCS stamping was unavailable. No Android APK was built.
- Independent source and test review passed.
- An existing shared-refresh-token fixture was made deterministic: it expires
  the original access tokens after login and still uses real fake-server refresh
  rotation. A one-second lifetime previously expired during race-instrumented
  login/refresh and failed before the intended assertion.

## Remaining performance work, in order

1. Integrate the reviewed engine commit through CloudBridge's immutable rclone
   pin and build a new signed test APK. Do not silently enable the stale metadata
   cache or disable checksum/account/baseline validation.
2. Measure startup scanning separately from per-file upload completion on a
   disposable Proton folder. Compare unchanged scans and batches of 100/1000
   small files with the previous engine; record file count, directory depth,
   effective flags, scan duration, request/retry counts, crypto CPU time and
   completed-files-per-second. Compare ordinary copy with bisync to isolate
   backend overhead from checksum/bisync work. Then repeat on Android.
3. Profile `SearchByNameInActiveFolder` and upload conflict handling. Authenticated
   name lookup currently decrypts every candidate in a fresh folder listing to
   reject modern/legacy-hash duplicates and type collisions. Repeated lookups can
   therefore perform quadratic name work as folders grow. This is a concrete
   code path worth profiling, not yet a measured cause of the reported upload
   speed. New-file creation is not assumed to enter the conflict path every time.
4. If that work dominates, design bounded reuse of successfully authenticated
   ciphertext names while keeping fresh candidate listings and current duplicate,
   type and hash decisions. Bind any reuse to actual parent and verifier key
   material; preserve time-dependent signature validation, cancellation and
   bounded plaintext memory. Test changed ciphertext/signers/parent keys,
   removals, duplicates and warm-cache corruption before implementing it. Do not
   use legacy hashes alone to infer absence or select an overwrite target.
5. Check saved tuning for bandwidth/API caps and profile per-file draft/key/block/
   commit requests, Android CPU/memory/storage, network latency and throttling.
   Adjust concurrency only after measurement. A throughput cap or more workers
   cannot by itself remove per-file crypto and API round trips. Batching or SDK
   changes require confirmed API support and preservation of conflict safeguards.

The local patch addresses the verified listing overhead. The published
v1.1.1-rc.1 APK and its engine pin have not changed during this investigation.
