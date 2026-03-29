#!/bin/bash
# Count Go LOC snapshot at each generation finished tag.
# This measures total code present after each run completed.

set -euo pipefail
cd "$(dirname "$0")"

echo "tag_base,date,go_loc_at_finish"

TOTAL=0
RUN_COUNT=0

for start_tag in $(git tag -l '*-start' --sort=creatordate); do
    base=$(echo "$start_tag" | sed 's/-start$//')
    finished_tag="${base}-finished"

    git rev-parse "$finished_tag" >/dev/null 2>&1 || continue

    commit_date=$(git log -1 --format='%ai' "$finished_tag" 2>/dev/null | cut -d' ' -f1)

    # Count all .go lines at the finished tag snapshot
    loc=$(git ls-tree -r --name-only "$finished_tag" 2>/dev/null | grep '\.go$' | grep -v mage_output_file.go | while read f; do
        git show "$finished_tag:$f" 2>/dev/null | wc -l
    done | awk '{s+=$1} END {print s+0}')

    TOTAL=$((TOTAL + loc))
    RUN_COUNT=$((RUN_COUNT + 1))

    short_name=$(echo "$base" | sed 's/generation-//')
    echo "$short_name,$commit_date,$loc"
done

echo ""
echo "TOTAL Go LOC summed across $RUN_COUNT generation snapshots: $TOTAL"
