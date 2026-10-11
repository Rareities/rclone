# Contained Proton API source

This directory preserves the exact previously pinned module
`github.com/jomplox/go-proton-api v1.0.5-0.20260907162606-9dcca00ba1dc`,
including its licence, tests and module metadata. Its module checksum was
`h1:yMj9021iQCZc+qkwx/JQWY/kZtb0Z4zRf/xr+aweJC8=`.

The existing refresh proposal is upstream `rclone/go-proton-api#10`, commit
`9dcca00ba1dc779a8958c9d4988b79ce2f9f3233`. The release integration retains the
additional reviewed `client.go`, `auth_hook_boundary_test.go` and
`crypto_integrity_test.go` from the saved fork implementation `fc35e4a8c752`:
same-UID and spent-token checks, bounded late-winner adoption, proper callback
lock ordering, persistence-error propagation and negative signature fixtures.
The caller checks persistence both before and after refresh-hook adoption.

No signature error is ignored and no encryption or SRP algorithm is replaced.
The local replacement makes the reviewed source reproducible instead of
depending on a moving branch. Remove it only after an upstream release contains
equivalent behaviour and passes the backend, fake-server, concurrent refresh,
negative crypto and disposable compatibility acceptance tests.
