# Generation Run 34

Date: 2026-03-15 to 2026-03-16
Issue: GH-1400
Tag: generation-gh-1400-run34-finished
Scaffold: v0.20260315.0

## Summary

Largest generation run to date. 246 stitch tasks, 37,542 LOC, $429 over ~24 hours of wall-clock time. Reached 65% of all requirements (710/1,091). Run was stopped due to GitHub API secondary rate limit exhaustion caused by stale worktree branch loop.

## Stats

| Metric | Value |
|--------|-------|
| Tasks | 246 done |
| LOC prod | +15,821 |
| LOC test | +21,721 |
| Total LOC | +37,542 |
| Cost | $429.10 (stitch $316.82 + measure $112.28) |
| Turns | 5,606 |
| Requirements | 710/1,091 (65%) |
| Releases | rel00.0 through rel06.0 |

## Findings

### Execution constitution (articles E6-E8)

Added build/test guidance, test planning, and repository conventions to the execution constitution as articles E6-E8. Initial placement as custom top-level fields caused schema validation errors and defect issues. Fixed by moving content into the articles array.

Results: eliminated catastrophic 31-turn outliers from test retry loops. Average turns unchanged (~15).

### CLAUDE.md not read by stitch agent

CLAUDE.md is loaded by interactive Claude Code sessions but not by stitch (which runs via `claude -p` with a pre-built prompt). Guidance must go in the execution constitution to reach the stitch agent.

### Generator premature stop bug (cobbler-scaffold#1475)

hasOpenIssues() race condition between two consecutive calls after closing the last task. Causes generator to skip measure AND stop in the same cycle, requiring manual restart. Happened on every batch boundary.

### Stale task branch loop (cobbler-scaffold#1561)

Computer suspend leaves orphaned task branches without worktrees. recoverStaleTasks misses them. createWorktree fails with exit 128 every 8 seconds in an infinite loop. Cascades into GitHub secondary rate limit exhaustion. Required manual `git branch -D` to fix.

### Measure watchdog kills (cobbler-scaffold#1509)

Opus thinking timeout on 146KB+ prompts with 73 PRDs. Killed after 20 minutes of no output. Intermittent — some cycles pass, others don't.

## Issues Filed

- cobbler-scaffold#1474: Execution constitution: add build/test guidance
- cobbler-scaffold#1475: Generator stops prematurely (hasOpenIssues race)
- cobbler-scaffold#1509: Measure killed by idle watchdog on large prompt
- cobbler-scaffold#1545: stats:generator should show ETA and cost estimate
- cobbler-scaffold#1561: Stitch loops forever on stale task branch
- cobbler-scaffold#1562: Generator should back off on repeated task failures
