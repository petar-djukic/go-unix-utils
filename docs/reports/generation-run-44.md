# Generation Run 44 Report

## Summary

Run 44 is the first full-catalog generation on cobbler-scaffold v0.20260513.0, running on the post-cleanup repository that was reset to specs-only on 2026-05-13 (PR #5057). The run produced 58,253 lines of Go code across 113 utilities in 412 tasks at a total cost of $997. Requirements coverage reached 52% (760 of 1,475) before the run was stopped.

## Results

| Metric | Run 42 | Run 44 |
|---|---|---|
| Date | 2026-04-06 to 2026-04-11 | 2026-05-13 to 2026-05-30 |
| Scaffold version | v0.20260406.0 | v0.20260513.0 |
| Requirements | 1,475/1,475 (100%) | 760/1,475 (52%) |
| Stitch tasks | 445 | 412 |
| Stitch invocations | — | 575 |
| LOC (prod) | 44,350 | 32,975 |
| LOC (test) | 31,043 | 25,278 |
| LOC (total) | 75,393 | 58,253 |
| Utilities touched | 113 | 113 |
| Total cost | $593.82 | $997.19 |
| Cost/requirement | $0.40 | $1.31 |

## Stitch Performance

| Metric | Value |
|---|---|
| Total invocations | 575 |
| Unique task IDs | 493 |
| Successful productive tasks | 392 |
| Failed tasks (never productive) | 101 |
| Tasks with retries | 15 |
| Max attempts for a single task | 18 |
| Total turns | 16,574 |
| Total rate-limited time | 54m |
| Tokens in (stitch) | 397.2M |
| Tokens out (stitch) | 6.4M |
| Duration total | 53.5h |
| Duration median | 205s |
| Duration p95 | 1,113s |
| Retry cost | $22.32 |
| Failed task cost | $174.58 |

## Cost Decomposition

Analysis of tool-call transcripts shows where cost and time are spent:

| Category | Cost share | Time share |
|---|---|---|
| Read code | 50% | 49% |
| "Reasoning" (no tool) | 39% | 13% |
| Write code | 3% | 12% |
| Other bash | 3% | 12% |
| Build | 1% | 1% |
| Lint | 1% | 2% |
| Test | 0% | 6% |
| Other | 2% | 6% |

87% of cost was spent on tasks that ultimately succeeded; 13% on tasks that never produced LOC. Write code and test take proportionally much more wall-clock time than cost because they are token-cheap but execution-slow. Reasoning is the inverse: 39% of cost but only 13% of time.

## Unit Economics

| Metric | Value |
|---|---|
| Cost per KLOC (production) | $43.89 |
| Cost per KLOC (test) | $54.04 |
| Cost per requirement completed | $1.77 |
| Cost per utility | $9.32 |

## Retry Analysis

Three tasks consumed the most retry attempts:

| Task | Attempts | Cost | Utility |
|---|---|---|---|
| 5428 | 18 | $1.53 | cmd/factor (srd065 R3.1-R3.4) |
| 5664 | 14 | $2.81 | cmd/pr (srd110 R3.1) |
| 5463 | 14 | $4.43 | cmd/fmt (srd070 R2.1-R5.1) |

The retry cost is low ($22.32 total) because the orchestrator cap killed attempts quickly. The 101 failed tasks cost $174.58, most of which were single-attempt failures with status=success but zero productive LOC.

## Deviations from Run 42

Run 44 cost 68% more than run 42 ($997 vs $594) while achieving only 52% requirements coverage. The per-requirement cost more than tripled ($1.31 vs $0.40). Several factors contributed:

1. The run started from a fully reset repository (0 LOC), so every task had to generate from scratch rather than building on existing code.

2. The 101 failed tasks consumed $175 with zero output. Many were status=success but produced no LOC delta, likely due to the same duplicate task bug (cobbler-scaffold#2123) observed in run 42.

3. The run was stopped before completion at 52% requirements coverage.

## Comparison with Previous Full Generations

| Run | Date | Reqs | LOC | Cost | $/req |
|---|---|---|---|---|---|
| Run 31 | 2026-03-10 | 807 | 17,651 | $236 | $0.29 |
| Run 34 | 2026-03-15 | 253 | 16,324 | $247 | $0.98 |
| Run 37 | 2026-03-18 | 972 | 25,923 | $349 | $0.36 |
| Run 38 | 2026-03-23 | 1,220 | 45,789 | $382 | $0.31 |
| Run 40b | 2026-03-29 | 1,486 | 41,341 | $509 | $0.34 |
| Run 42 | 2026-04-06 | 1,475 | 44,350 | $525 | $0.36 |
| Run 44 | 2026-05-13 | 760 | 32,975 | $997 | $1.31 |
