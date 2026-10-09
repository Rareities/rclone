#!/usr/bin/env bash
set -euo pipefail
kind="$1"
case "$kind" in
  official) branch='automation/review-official-rclone'; title='chore: review official rclone master' ;;
  oss) branch='automation/review-oss-protondrive'; title='chore: review OSS Proton Drive fixes' ;;
  *) exit 2 ;;
esac
git config user.name 'github-actions[bot]'
git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
git remote add official https://github.com/rclone/rclone.git
git fetch --no-tags official master
official_sha="$(git rev-parse FETCH_HEAD)"
git switch -C "$branch" master
git merge --no-edit "$official_sha" || { git merge --abort || true; exit 1; }
if [ "$kind" = oss ]; then
  url='https://oss-oo.io/OSS-Singularity/rclone.git'
  git remote add oss "$url"
  oss_ref="$(git ls-remote --symref oss HEAD | awk '$1=="ref:" && $3=="HEAD" {print $2; exit}')"
  case "$oss_ref" in refs/heads/*) ;; *) echo 'OSS default branch unavailable' >&2; exit 1 ;; esac
  git fetch --no-tags oss "$oss_ref"
  oss_sha="$(git rev-parse FETCH_HEAD)"
  git merge-base "$official_sha" "$oss_sha" >/dev/null || exit 1
  mapfile -t commits < <(git rev-list --reverse --no-merges "$oss_sha" "^$official_sha" -- backend/protondrive)
  [ "$(( ${#commits[@]} ))" -le 40 ] || { echo 'Too many OSS commits for automatic review' >&2; exit 1; }
  for sha in "${commits[@]}"; do
    while IFS= read -r path; do
      case "$path" in backend/protondrive/*|docs/content/protondrive.md|go.mod|go.sum) ;;
      *) echo "Commit $sha changes non-allowlisted file $path" >&2; exit 1 ;;
      esac
    done < <(git diff-tree --no-commit-id --name-only -r "$sha")
    git cherry-pick -x --empty=drop "$sha" || { git cherry-pick --abort || true; exit 1; }
  done
fi
if git diff --quiet master HEAD; then
  echo 'changed=false' >> "$GITHUB_OUTPUT"
  exit 0
fi
echo 'changed=true' >> "$GITHUB_OUTPUT"
{
  echo "Review-only candidate: $kind"
  echo
  echo "Official master commit: $official_sha"
  if [ "$kind" = oss ]; then echo "OSS Forgejo commit: $oss_sha"; fi
  echo
  echo 'Requires manual source review and real disposable-data Proton Drive / bisync validation before merging. The automation never merges master.'
} > /tmp/upstream-pr.md
