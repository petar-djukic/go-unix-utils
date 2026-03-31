# Generation Run 40b Report

## Summary

Run 40b is the first full-catalog generation on cobbler-scaffold v0.20260328.1, covering all 33 releases (rel00.0 through rel15.0) and achieving 100% requirement completion (1,475/1,475). The run produced 107 commands and 6 shared packages totaling 76,933 lines of Go code.

## Results

| Metric | Run 34 | Run 38 | Run 40b |
|---|---|---|---|
| Date | 2026-03-16 | 2026-03-23 | 2026-03-28 |
| Scaffold version | v0.20260315.0 | v0.20260323.0 | v0.20260328.1 |
| Requirements | 710/1,091 (65%) | 1,395/1,475 (95%) | 1,475/1,475 (100%) |
| Stitch tasks | 246 | 323 | 403 (428 total, 25 failed) |
| Measure cycles | — | — | 113 |
| LOC (prod) | — | 45,789 | 41,341 |
| LOC (test) | — | 27,859 | 35,592 |
| LOC (total) | 37,542 | 73,648 | 76,933 |
| Commands | — | 105 | 107 |
| Shared packages | — | 6 | 6 |
| Total cost (stitch) | — | — | $480 |
| Total cost (measure) | — | — | ~$41 |
| Total cost | $429 | $432 | ~$521 |
| Cost/requirement | $0.60 | $0.29 | $0.35 |
| Wall time | — | ~4.2 days | ~24 hours |

## Stitch Performance

| Metric | Value |
|---|---|
| Avg time/task | 3.7 min |
| Median time/task | 2.3 min |
| Total stitch compute time | 24.8 hours |
| Avg turns/task | 21.2 |
| Avg cost/task | $1.19 |
| Avg tokens in/task | 842,929 |
| Avg tokens out/task | 12,752 |
| Total tokens in | 339,700,508 |
| Total tokens out | 5,138,918 |
| Tasks >5 min | 89 (22.1%) |
| Tasks >10 min | 28 (6.9%) |

## Slowest Tasks

These tasks took over 5 minutes and indicate requirements whose weights should be increased to prevent batching with other requirements.

| Time | Cost | Turns | Task | PRD |
|---|---|---|---|---|
| 11m17s | $2.50 | 36 | #3768 cmd/wc null delimiters, column alignment | prd005 R2.5-R3.2 |
| 10m35s | $2.15 | 22 | #3770 cmd/wc totals, GNU compat, SIGPIPE | prd005 R4.4-R6.1 |
| 10m15s | $1.95 | 26 | #3852 cmd/fold core implementation | prd023 R1.1-R1.4 |
| 9m37s | $2.54 | 48 | #3769 cmd/wc selective display, error handling | prd005 R3.3-R4.3 |
| 9m10s | $1.79 | 19 | #3790 cmd/ls core implementation | prd008 R1.1-R1.4 |
| 7m57s | $2.46 | 41 | #3804 cmd/ls tests and edge cases | prd008 R4.1-R4.4 |
| 7m32s | $2.17 | 45 | #3800 cmd/ls classification and indicators | prd008 R3.4-R3.7 |
| 7m17s | $1.63 | 16 | #3798 cmd/ls metadata display | prd008 R2.11-R2.14 |

The most time-consuming utilities were wc (3 tasks >9 min), ls (4 tasks >7 min), and fold (10 min). These PRDs contain requirements that are underweighted relative to their implementation complexity. Filed as #4206 for weight adjustment.

## Incidents

### Run 39 abort (pre-run)

The initial attempt (run 39) was aborted because the measure agent generated rel15.0 commands (ts) before rel11.1 commands (users, who). Root cause: use case files retained their original release prefixes (rel05.5, rel12.1, rel13.1) despite being moved to rel15.0 in the roadmap. Fixed in PR #3651 by renaming all files to rel15.0-uc00N.

### Releases config (pre-run)

The `configuration.yaml` releases list only included 11.1-15.0, omitting 00.0-10.0. Since `generator:start` resets all Go sources to 0 LOC, all releases must be present for full regeneration. Fixed by adding all 33 releases.

### Cycle limit hits (mid-run)

The generator hit the 100-cycle limit three times, requiring manual `resume` each time. Increased to 500 cycles in the worktree config. The zero-LOC detection (3 consecutive cycles with no LOC change) also triggered false stops when a failing task blocked progress.

### Task #4165 prd107-dir R2.4 failure

Failed 3 times with "Claude failure" (11s, 2s, 2s durations). Caused by rate limiting after ~12 hours of continuous generation, not a code problem. Succeeded on next resume after rate limits reset. Filed as #4167.

### Task #4205 prd111-ptx R5.1, R5.2

The final 2 requirements took multiple attempts spanning ~45 minutes of stitch time. The task combined exit code and SIGPIPE handling with differential tests that required matching gptx's permuted index output format. The stitch agent spent most of its time debugging output format mismatches. Weight was increased from 1 to 4 for both requirements to force individual scheduling. Succeeded on the final attempt. Filed as #4206 for systematic weight review.

## Changes Made During Run

- Renamed rel15.0 use case files (PR #3651)
- Removed rel13.1 from configured releases (no longer in roadmap)
- Added all releases 00.0-15.0 to configuration.yaml
- Upgraded cobbler-scaffold v0.20260328.0 → v0.20260328.1 (PR #3654)
- Increased generation cycles 100 → 500
- Increased prd111-ptx R5.1 and R5.2 weights from 1 to 4

## Issues Filed

- #4167: Investigate prd107-dir R2.4 stitch failure
- #4206: Increase weight for requirements that cause long stitch times
- cobbler-scaffold#1964: Analyze: detect use case ID prefix vs. roadmap release mismatch
- cobbler-scaffold#1966: Add mage stats:run target to extract generation run statistics
- cobbler-scaffold#1967: stats:generator: show attempt count per task and aggregate retry totals

## Comparison with Run 38

Run 40b achieved 100% requirements (vs 95% in run 38) and completed in ~24 hours wall time (vs ~4.2 days). Cost per requirement increased slightly ($0.35 vs $0.29), driven by the additional rel15.0 utilities (ts, stty, pr, ptx) which are algorithmically complex, and by retry overhead on failing tasks.

The test-to-prod LOC ratio improved from 0.61 (run 38) to 0.86 (run 40b), indicating the scaffold is generating more comprehensive differential tests per command.

Total LOC increased from 73,648 to 76,933 (+4.5%) while covering 2 additional commands (107 vs 105) and achieving full requirement coverage.
