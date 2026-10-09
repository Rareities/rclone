# Dual-upstream maintenance

This is a native fork of [rclone/rclone](https://github.com/rclone/rclone). The default `master` branch remains based on official rclone. The secondary Proton Drive source is [OSS-Singularity on Forgejo](https://oss-oo.io/OSS-Singularity/rclone).

## Integration strategy

- Official upstream is checked daily against `rclone/rclone:master`.
- Separately, the OSS updater discovers Forgejo's default branch and selectively proposes non-merge commits touching `backend/protondrive/` and no files outside `backend/protondrive/`, `docs/content/protondrive.md`, `go.mod` or `go.sum`.
- It also applies any pending official rclone updates to the OSS candidate, so the candidate is tested against an up-to-date official base. If both pull requests exist, merge the official update first and regenerate the OSS candidate.
- Each candidate is compiled and tested (`backend/protondrive` and `cmd/bisync`) before a **draft** pull request is opened.
- Nothing is automatically merged into `master`. Conflicts, unrelated histories, many commits or unavailable OSS hosting fail visibly.
- Other OSS fixes, commits changing unrelated files, dependency-only commits and patch equivalents already upstream may need manual selection. Successful CI is not proof of production-safe Proton Drive syncing.

## Enabling the automation

1. Merge the setup pull request after reviewing the workflow.
2. Open [repository Actions settings](https://github.com/Rareities/rclone/settings/actions) and ensure Actions are enabled for this fork.
3. Under workflow permissions, enable **Allow GitHub Actions to create and approve pull requests**. The workflow creates drafts; it does not approve or merge them.
4. Run **Review official rclone and OSS Proton patches** in the [Actions tab](https://github.com/Rareities/rclone/actions) with source `both`. Check logs, especially Forgejo access.
5. Manually review candidate source commits, dependencies and tests. Test `bisync` on disposable files, including first `--resync`, removals, renames, interrupted runs, conflicts and recovery.

## Limitations

The Forgejo URL is configured but has not been verified reachable from the execution environment used to prepare this setup. Its runner access and precise patch history will only be known when Actions runs. This repository does not currently claim to include OSS changes.

The workflow creates only proposed pull requests; it does not touch CloudBridge. To consume a reviewed `Rareities/rclone` revision in CloudBridge, update `de.schuelken.cloudbridge.rCloneRepoUrl` and pin `de.schuelken.cloudbridge.rCloneRef` to a **tested commit SHA**. Do not pin it to a moving branch. The Android app still needs actual bisync user-interface and worker support.
