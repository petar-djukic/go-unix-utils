# Generation Run 36

- Date: 2026-03-17
- Issue: GH-2626
- Branch: generation-gh-2626-run36 (aborted, not merged)
- Tags: generation-gh-2626-run36-start
- Scaffold: v0.20260317.1

## Configuration

- cycles: 6
- max_measure_issues: 3
- max_stitch_issues_per_cycle: 3 (first run with multi-stitch)
- max_requirements_per_task: 4
- measure_source_mode: headers

## Purpose

Continue the multi-issue measure investigation from run 35. Run 35 set max_measure_issues=3 but left max_stitch_issues_per_cycle=1, creating a bottleneck: measure proposed 2-3 tasks per cycle, but stitch could only process 1. Run 36 sets both to 3 to test full parallel throughput.

## Results

| Metric | Value |
|--------|-------|
| Tasks completed | 13 (+ 2 killed mid-flight) |
| Measure cycles | 6 (3 invocations logged) |
| Cost (recorded) | $1.55 (stitch $0.62 + measure $0.93) |
| Wall clock | ~33 minutes |
| PRDs touched | prd001-testutils (partial), prd002-sys (done), prd003-format (done) |
| Releases completed | none (aborted before rel00.0 gate) |

Cost is underreported: only 1 of 13 stitch tasks recorded cost in history (the rest completed before the orchestrator wrote history, or history was lost when the run was killed).

## Cycle Summary

| Cycle | Measure | Stitch Tasks | Wall Time | Notes |
|-------|---------|-------------|-----------|-------|
| 1 | 3 issues | 3 (contracts: #2632, #2633, #2634) | 2m37s | API stub tasks, fast |
| 2 | 3 issues | 3 (#2636, #2637, #2638) | 7m01s | Core implementations |
| 3 | 3 issues | 3 (#2640, #2641, #2642) | 7m51s | Continued implementations |
| 4 | 3 issues | 3 (#2644, #2645, #2646) | 3m43s | Remaining exports |
| 5 | 3 issues | 1 done (#2648), 1 killed (#2649) | 3m48s | Run stopped mid-cycle |
| 6 | — | — | — | Never reached |

## Multi-Issue Measure + Multi-Stitch Findings

### Throughput improvement

| Metric | Run 34 (1×1) | Run 35 (3×1) | Run 36 (3×3) |
|--------|-------------|-------------|-------------|
| Tasks per cycle | 1 | 0.83 | 2.6 |
| Tasks per hour | 10.25 | — | 23.6 |
| Measure tasks proposed | 1 | 2 (3rd deduped) | 3 |
| Stitch tasks per cycle | 1 | 1 (bottleneck) | 3 |

Run 36 achieves 2.3× the throughput of run 34 in tasks per hour. The improvement comes from two sources: measure proposes 3 tasks per cycle instead of 1, and stitch processes all 3 sequentially within the same cycle.

### Measure dedup fix

Run 35 reported that the 3rd measure iteration almost always produced a duplicate rejected by the dedup filter (cobbler-scaffold#1602). Run 36 on scaffold v0.20260317.1 successfully produced 3 distinct tasks per measure cycle. Either the dedup issue was fixed in v0.20260317.1 or the early-run task space (3 independent PRDs with clear boundaries) made overlap less likely. Needs more data at higher task counts to confirm.

### Stitch remains sequential

Even with max_stitch_issues_per_cycle=3, the orchestrator processes stitches sequentially (each gets its own temp worktree, runs Claude, merges, then starts the next). The 3× throughput comes from avoiding measure overhead between each stitch, not from parallelism. True parallel stitch would require concurrent worktrees and separate Claude sessions.

### Cost efficiency

Measure cost is amortized across 3 tasks instead of 1. In run 34, measure cost $0.46 per task ($112.28 / 246). In run 36, measure cost $0.93 for 6 cycles producing ~18 tasks = $0.05 per task proposed. This is a 9× reduction in measure cost per task, though the small sample size makes this estimate rough.

## Issues Filed

- cobbler-scaffold#1608: generator:start should create its own git worktree

## Why Aborted

Run was stopped to investigate the worktree workflow. The generation ran directly in the main repo instead of a worktree (operator error — generator:start does not create a worktree automatically). Filed cobbler-scaffold#1608 for the root cause. The generation branch was deleted after extracting stats; the start tag preserves the reference point.
