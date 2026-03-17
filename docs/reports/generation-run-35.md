# Generation Run 35

- Date: 2026-03-17
- Issue: GH-2607
- Branch: generation-gh-2607-run35
- Tags: generation-gh-2607-run35-start, generation-gh-2607-run35-finished
- Scaffold: v0.20260317.0

## Configuration

- cycles: 6
- max_measure_issues: 3 (first run with multi-issue measure)
- max_stitch_issues_per_cycle: 1
- max_requirements_per_task: 4
- measure_source_mode: headers

## Results

- Tasks completed: 5
- LOC produced: 328 prod + 229 test = 557
- PRDs touched: prd001-testutils (partial)
- Releases completed: none (prd001-testutils still in progress)

## Cycle Summary

| Cycle | Stitch | Measure | LOC Delta |
|-------|--------|---------|-----------|
| 1 | 0 (no issues) | 2 issues (#2615, #2616) | 0 |
| 2 | 1 (#2615) | skipped | +282 prod |
| 3 | 1 (#2616) | 2 issues (#2618, #2619) | +15 prod |
| 4 | 1 (#2618) | skipped | 0 |
| 5 | 1 (#2619) | 2 issues (#2621, #2624) | +229 test |
| 6 | 1 (#2621) | 2 issues (#2623, #2624) | +31 prod |

## Multi-Issue Measure Observations

With max_measure_issues=3, the orchestrator makes 3 sequential Claude calls with
tasksPerCall=1. Each iteration sees the previous iteration's issues as context.
The 3rd iteration almost always proposes a duplicate (overlapping output files)
that gets rejected by the dedup filter. Net result: 2 issues per measure instead
of the requested 3.

Filed cobbler-scaffold#1602: tasksPerCall=1 defeats multi-issue measure.

## New Constitution Articles

This was the first run with cobbler-scaffold v0.20260317.0, which adds articles
E6-E10 to execution.yaml: non_goals enforcement, skip-on-ambiguity, 40-statement
function limit, 500-line file limit, DRY enforcement. The measure prompt now
proposes contract tasks (e.g. "prd002-sys (contract): exported types and function
signatures").

## Issues Filed

- cobbler-scaffold#1602: Multi-issue measure uses tasksPerCall=1
