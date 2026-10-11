#!/usr/bin/env bash
set -euo pipefail

# Runs only from the trusted master checkout in a fresh write-scoped job.
kind="${1:-}"
phase="${2:-}"
case "$kind" in
  official)
    branch='automation/review-official-rclone'
    title='chore: review official rclone master'
    ;;
  oss)
    branch='automation/review-oss-protondrive'
    title='chore: review OSS Proton Drive fixes'
    ;;
  *) echo 'Unknown review kind' >&2; exit 2 ;;
esac
case "$phase" in validate-push|pr) ;; *) exit 2 ;; esac

artifact="${RUNNER_TEMP:?}/review-artifact"
body="$artifact/pr-body.md"
bundle="$artifact/candidate.bundle"
meta="$artifact/refs.txt"

if [ "$phase" = pr ]; then
  # Only gh sees GH_TOKEN, supplied by the workflow in this separate step.
  test -f "$body"
  test "$(wc -c < "$body")" -le 16384
  existing="$(gh pr list --head "$branch" --base master --state open --json number --jq '.[0].number // empty')"
  if [ -z "$existing" ]; then
    gh pr create --draft --base master --head "$branch" --title "$title" --body-file "$body"
  else
    gh pr edit "$existing" --body-file "$body"
  fi
  exit 0
fi

# Artifact files are data. Never source/eval them or check out bundle objects.
test -f "$body" && test -f "$bundle" && test -f "$meta"
test ! -L "$body" && test ! -L "$bundle" && test ! -L "$meta"
test "$(wc -c < "$bundle")" -le 104857600
test "$(wc -c < "$body")" -le 16384
test "$(wc -c < "$meta")" -le 1024
mapfile -t expected < "$meta"
test "${#expected[@]}" -eq 5
test "${expected[0]}" = "$kind"
for i in 1 2; do
  [[ "${expected[i]}" =~ ^[a-f0-9]{40}$ ]] || exit 1
done

# Checkout is master with persisted checkout credentials; no candidate code runs.
test "$(git branch --show-current)" = master
git fetch --no-tags origin +refs/heads/master:refs/remotes/origin/master
base_sha="$(git rev-parse refs/remotes/origin/master)"
test "$base_sha" = "$(git rev-parse HEAD)"
test "$base_sha" = "${expected[1]}"

official_sha="$(git ls-remote https://github.com/rclone/rclone.git refs/heads/master | awk '$2=="refs/heads/master" {print $1}')"
[[ "$official_sha" =~ ^[a-f0-9]{40}$ ]] || exit 1
test "$official_sha" = "${expected[2]}"

if [ "$kind" = oss ]; then
  url='https://oss-oo.io/OSS-Singularity/rclone.git'
  discovered="$(git ls-remote --symref "$url" HEAD)"
  oss_ref="$(printf '%s\n' "$discovered" | awk '$1=="ref:" && $3=="HEAD" {print $2}')"
  oss_sha="$(printf '%s\n' "$discovered" | awk '$2=="HEAD" && $1!="ref:" {print $1}')"
  [[ "$oss_ref" =~ ^refs/heads/[A-Za-z0-9._/-]+$ ]] || exit 1
  [[ "$oss_sha" =~ ^[a-f0-9]{40}$ ]] || exit 1
  test "$oss_ref" = "${expected[3]}"
  test "$oss_sha" = "${expected[4]}"
  git fetch --no-tags "$url" "$oss_ref"
  test "$(git rev-parse FETCH_HEAD)" = "$oss_sha"
else
  test "${expected[3]}" = none && test "${expected[4]}" = none
fi

git bundle verify "$bundle"
candidate_ref="refs/remotes/review-candidate/$kind"
git -c core.hooksPath=/dev/null fetch --no-tags "$bundle" "refs/heads/$branch:$candidate_ref"
head_sha="$(git rev-parse "$candidate_ref")"
[[ "$head_sha" =~ ^[a-f0-9]{40}$ ]] || exit 1
git merge-base --is-ancestor "$base_sha" "$head_sha"
git merge-base --is-ancestor "$official_sha" "$head_sha"
test "$base_sha" != "$head_sha"
test "$(git rev-list --count "$base_sha..$head_sha")" -le 5000

# For OSS, all patches after the official merge must stay within the historical
# selective cherry-pick allowlist; candidate must still be reviewed by a human.
if [ "$kind" = oss ]; then
  while IFS= read -r sha; do
    test "$(git rev-list --parents -n 1 "$sha" | wc -w)" -eq 2 || {
      echo 'Unexpected merge in OSS patch tail' >&2; exit 1;
    }
    while IFS= read -r path; do
      case "$path" in
        backend/protondrive/*|docs/content/protondrive.md|go.mod|go.sum) ;;
        *) echo "OSS candidate contains non-allowlisted patch" >&2; exit 1 ;;
      esac
    done < <(git diff-tree --no-commit-id --name-only -r "$sha")
    origin_sha="$(git log -1 --format=%B "$sha" |
      sed -nE 's/^\(cherry picked from commit ([a-f0-9]{40})\)$/\1/p' | tail -n 1)"
    [[ "$origin_sha" =~ ^[a-f0-9]{40}$ ]] || {
      echo 'OSS candidate lacks a source commit reference' >&2; exit 1;
    }
    git merge-base --is-ancestor "$origin_sha" "$oss_sha"
    candidate_patch="$(git show --no-ext-diff --format= "$sha" | git patch-id --stable | awk '{print $1}')"
    upstream_patch="$(git show --no-ext-diff --format= "$origin_sha" | git patch-id --stable | awk '{print $1}')"
    test -n "$candidate_patch" && test "$candidate_patch" = "$upstream_patch" || {
      echo 'OSS candidate patch differs from its source commit' >&2; exit 1;
    }
  done < <(git rev-list --reverse --first-parent "$head_sha" "^${official_sha}" "^${base_sha}" --no-merges)
fi

# Read the old head before updating; force-with-lease does not overwrite any
# branch modified since this inspection. Git uses trusted checkout credentials.
remote_head="$(git ls-remote origin "refs/heads/$branch" | awk '{print $1}')"
if [ -n "$remote_head" ]; then
  [[ "$remote_head" =~ ^[a-f0-9]{40}$ ]] || exit 1
fi
git push --force-with-lease="refs/heads/$branch:$remote_head" origin "$head_sha:refs/heads/$branch"
