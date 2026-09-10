#!/usr/bin/env bash
# Record the check-evaluation verdict this binary produces for every real consumer repo.
#
# The characterization target is the IS_STATUS_SUCCESS column: it is computed purely by
# hydrateCommits from the requested status_check_names, independent of the expression. So we run
# with an expression that can never be true, which means:
#   - no commit ever "passes", so the promotion branch is never taken (belt to --dry-run's braces)
#   - the loop is never cut short by a success, so every examined commit appears in the table
# Combined with the non-ancestor probe destinations in consumers.tsv, this prints a full table of
# per-commit verdicts, which is exactly what must not change across a refactor.
#
#   ./record.sh <path-to-github-flow-manager-binary> <output-dir>
#
# Compare a refactor against the recorded baseline:
#   ./record.sh ./gfm-new /tmp/out-new && diff -ru testdata/baseline/v1.1.10 /tmp/out-new
#
# The verdict depends on live GitHub CI state, so a repo whose CI moved between the two runs will
# differ for reasons unrelated to the code. Each output records the SHAs it saw so drift is visible
# rather than silent — re-record a moved repo, don't explain the diff away.
set -uo pipefail

BIN="${1:?usage: record.sh <binary> <output-dir>}"
OUT="${2:?usage: record.sh <binary> <output-dir>}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMMITS="${COMMITS:-25}" # match whatever your promote workflow passes as --commits-number

# Deliberately unsatisfiable: see header.
EXPRESSION='Message contains "___gfm_baseline_never_match___"'

: "${GITHUB_TOKEN:="$(gh auth token 2>/dev/null)"}"
if [ -z "$GITHUB_TOKEN" ]; then
  echo "GITHUB_TOKEN unset and 'gh auth token' produced nothing" >&2
  exit 1
fi
export GITHUB_TOKEN

mkdir -p "$OUT"

while IFS=$'\t' read -r repo source dest checks; do
  case "$repo" in '' | '#'*) continue ;; esac

  echo "recording $repo ..." >&2
  {
    echo "# repo=$repo source=$source destination=$dest commits=$COMMITS"
    echo "# expression=$EXPRESSION"
    echo "# status_check_names=$checks"
    echo "#"
    "$BIN" DocPlanner "$repo" "$source" "$dest" "$EXPRESSION" "$checks" \
      -c "$COMMITS" --dry-run --verbose 2>&1
    echo "# exit=$?"
  } >"$OUT/$repo.txt"
done <"$HERE/consumers.tsv"

echo "wrote $(find "$OUT" -name '*.txt' | wc -l | tr -d ' ') files to $OUT" >&2
