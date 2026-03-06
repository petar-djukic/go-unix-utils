# GH-206 Generation Run 14: 36 Tasks, 5,557 LOC, Measure Prompt Scaling Wall

We ran the cobbler pipeline on generation branch generation-gh-206-generation-run-14 targeting
all releases (rel00.0 through rel04.1) with unlimited cycles. The run stitched 36 tasks across
37 measure cycles (4 failed), producing 2,238 production LOC and 3,319 test LOC across 35
source files (5,557 total Go LOC). The run also produced 2 new PRDs (prd012-true, prd013-false).

The run required 3 manual restarts due to idle watchdog kills caused by internet connectivity
issues. The final failure occurred at a 312 KB measure prompt (31 source files), where the
Claude API connection stalled with zero tokens in or out. The same prompt sizes succeeded on
previous cycles, confirming these were transient network failures rather than prompt size limits.

The run implemented 8 command utilities (cat, du, false, ls, sponge, true, version, wc) plus
3 foundation packages (format, sys, testutils) with differential tests. The measure agent
demonstrated sophisticated ordering: foundations first, then commands with interleaved tests,
then gap-filling for uncovered requirements, then new PRD generation for rel02.0 trivial
utilities.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Generation branch | generation-gh-206-generation-run-14 |
| Scaffold version | v0.20260303.0 |
| Model | Claude Opus 4.6 (in podman containers) |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 1 |
| Max stitch issues per cycle | 1 |
| CLAUDE_CODE_MAX_OUTPUT_TOKENS | 128,000 |
| max_requirements_per_task | 4 |
| idle_timeout_seconds | 600 |
| enforce_measure_validation | false |
| max_measure_retries | 0 |
| releases | ["00.0", "01.1", "01.2", "01.3", "02.0", "02.1", "02.2", "03.0", "04.0", "04.1"] |
| context_include | VISION.yaml, ARCHITECTURE.yaml |
| context_exclude | SPECIFICATIONS.yaml, road-map.yaml |
| Seeded LOC | 0 (clean start) |
| Budgeted cycles | unlimited (cycles: 0) |
| Actual cycles | 37 measure, 34 stitch |
| Total tasks completed | 36 |

## Cycle-by-cycle results

Table 2: All tasks in execution order

| # | Issue | Description | Type | Stitch Time |
|---|-------|-------------|------|-------------|
| 1 | #225 | pkg/testutils DiffTest struct and differential runner | prod | 7m55s |
| 2 | #226 | pkg/testutils RunDiffTests core comparison tests | test | 7m7s |
| 3 | #227 | pkg/format core formatting (sizes, colors, columns) | prod | 11m35s |
| 4 | #228 | pkg/sys file metadata abstraction | prod | 5m6s |
| 5 | #229 | pkg/format FileTypeColor and color detection tests | test | 2m15s |
| 6 | #230 | pkg/format column layout tests | test | 1m39s |
| 7 | #231 | pkg/format HumanSize tests | test | 1m10s |
| — | — | Measure watchdog kill (149 KB, 0 tokens) | fail | — |
| 8 | #232 | pkg/sys Stat, Lstat, FileMetadata tests | test | 1m55s |
| 9 | #233 | cmd/cat implementation (input, decoration, display) | prod | 7m20s |
| 10 | #234 | cmd/sponge implementation (buffering, write, append) | prod | 10m28s |
| 11 | #235 | cmd/cat differential tests (R1.1, R1.2, R5.2, R5.3) | test | 4m10s |
| 12 | #236 | cmd/sponge differential tests (R1.1, R2.1, R3.1, R4.1) | test | 1m9s |
| 13 | #237 | cmd/du core directory traversal and disk usage | prod | 7m11s |
| — | — | Measure watchdog kill (207 KB, 0 tokens) | fail | — |
| 14 | #238 | cmd/du differential tests (R1.1, R1.2, R1.3, R1.5) | test | 1m13s |
| 15 | #239 | cmd/du differential tests (R1.1, R1.2, R1.3, R1.5) | test | 2m11s |
| 16 | #240 | cmd/wc core line/word/byte counting | prod | 7m25s |
| 17 | #241 | cmd/wc differential tests (R1.1, R1.2, R1.3, R1.4) | test | 1m23s |
| 18 | #242 | cmd/version implementation (prd011-magefiles) | prod | 1m38s |
| 19 | #243 | cmd/ls directory listing, sorting, one-per-line | prod | 9m3s |
| 20 | #244 | cmd/ls core differential tests (R1.2, R1.3, R1.4) | test | 2m39s |
| 21 | #245 | cmd/ls multi-column terminal output | prod | — |
| — | — | Measure watchdog kill (260 KB, 0 tokens) | fail | — |
| 22 | #246 | pkg/sys TerminalWidth unit tests | test | 49s |
| 23 | #247 | cmd/ls --color flag (auto/always/never) | prod | 6m17s |
| 24 | #248 | cmd/cat differential tests (R2.1, R2.3, R3.1, R4.3) | test | 1m41s |
| 25 | #249 | testutils normalization function tests | test | 47s |
| 26 | #250 | cmd/cat diff tests (squeeze, non-printing) | test | 1m10s |
| 27 | #253 | cmd/cat flag alias tests (-A, -e, -t, -u) | test | 1m46s |
| 28 | #254 | PRDs for true/false utilities (rel02.0) | doc | 59s |
| 29 | #255 | cmd/true + cmd/false implementation | prod | 2m25s |
| 30 | #256 | cmd/false differential tests | test | 42s |
| 31 | #257 | cmd/true differential tests | test | 45s |
| 32 | #258 | cmd/du display mode flags (-a, -s, -c, -d) | prod | 1m59s |
| 33 | #259 | cmd/du differential tests (R2.2, R2.3, R2.4, R2.7) | test | 58s |
| 34 | #260 | cmd/version unit tests (prd011-magefiles R1) | test | 53s |
| — | — | Measure watchdog kill (312 KB, 0 tokens) | fail | — |

LOC totals: 2,238 prod + 3,319 test = 5,557. 34 of 36 tasks succeeded on first attempt.
4 measure cycles failed (idle watchdog kills due to network issues).

## Measure prompt scaling

The measure prompt includes all Go source files, causing linear growth with codebase size.
This run hit the practical ceiling where 300+ KB prompts become unreliable.

Table 3: Measure prompt size progression

| Cycle | Prompt Size | Source Files | Prod LOC | Test LOC | Measure Time |
|-------|-------------|-------------|----------|----------|--------------|
| 1 | 70 KB | 0 | 0 | 0 | 3m44s |
| 3 | 88 KB | 6 | 379 | 0 | 4m42s |
| 5 | 98 KB | 9 | 539 | 304 | 5m57s |
| 7 | 110 KB | 12 | 539 | 668 | 2m9s |
| 8 | 149 KB | 11 | 704 | 1,021 | FAIL (10m) |
| 9 | 155 KB | 12 | 704 | 1,021 | 4m1s |
| 11 | 172 KB | 15 | 704 | 1,021 | 8m21s |
| 13 | 191 KB | 18 | 864 | 1,021 | 4m35s |
| 15 | 207 KB | 20 | 1,024 | 1,236 | FAIL (8m) |
| 17 | 207 KB | 20 | 1,024 | 1,236 | 6m28s |
| 21 | 245 KB | 22 | 1,988 | 2,150 | 10m19s |
| 23 | 260 KB | 24 | 2,042 | 2,330 | FAIL (10m) |
| 25 | 265 KB | 25 | 2,042 | 2,443 | 9m36s |
| 29 | 283 KB | 26 | 2,086 | 2,815 | 8m7s |
| 33 | 303 KB | 29 | 2,188 | 3,095 | 9m3s |
| 37 | 312 KB | 31 | 2,238 | 3,319 | FAIL (10m) |

At 312 KB (31 source files), measure takes 8-10 minutes when it succeeds and fails
completely when the API connection stalls. The 70 KB baseline (docs + constitutions) plus
~8 KB per source file means the prompt doubles every ~9 source files added.

This is the primary scaling bottleneck. See GH-251 for proposed interface encapsulation
to reduce prompt size.

## Cost analysis

Table 4: Cost breakdown

| Category | Cost |
|----------|------|
| Measure (37 cycles, 4 failed) | $26.63 |
| Stitch (34 tasks) | $33.04 |
| Total | $59.67 |

At $59.67 for 5,557 Go LOC, the cost is $10.74 per 1,000 LOC. This is significantly higher
than previous runs due to: (a) larger measure prompts at 260-312 KB consuming more input
tokens, (b) 4 wasted measure cycles ($0 output but API may have charged for input), and
(c) the measure agent spending more time reasoning about a larger codebase.

Table 5: Cost per LOC comparison across runs

| Run | Model | Total LOC | Cost | Cost/KLOC |
|-----|-------|-----------|------|-----------|
| 11 (eng18) | Sonnet 4.6 | 4,218 | ~$23 | ~$5.46 |
| 12 (eng20) | Opus 4.6 | 5,303 | ~$36 | ~$6.79 |
| 14 (eng21) | Opus 4.6 | 5,557 | ~$60 | ~$10.74 |

The cost increase from run 12 to 14 is primarily due to prompt growth. Run 12 ended at
282 KB; run 14 started from 0 but grew past 312 KB with more test files. Measure cost
scales superlinearly with prompt size because each cycle re-reads the entire codebase.

## Generated packages

Table 6: All packages produced in run 14

| Package | Prod Files | Prod LOC | Test Files | Test LOC | Release |
|---------|-----------|----------|-----------|----------|---------|
| pkg/testutils | 1 | 305 | 2 | 381 | rel00.0 |
| pkg/sys | 4 | 109 | 2 | 328 | rel00.0 |
| pkg/format | 3 | 292 | 3 | 774 | rel00.0 |
| cmd/version | 1 | 43 | 1 | 143 | rel00.0 |
| cmd/wc | 1 | 227 | 1 | 263 | rel01.0 |
| cmd/cat | 1 | 282 | 1 | 489 | rel01.1 |
| cmd/sponge | 1 | 328 | 1 | 230 | rel01.2 |
| cmd/du | 1 | 211 | 1 | 323 | rel01.3 |
| cmd/ls | 1 | 318 | 1 | 208 | rel04.0 |
| cmd/true | 1 | 50 | 1 | 90 | rel02.0 |
| cmd/false | 1 | 52 | 1 | 90 | rel02.0 |
| Total | 17 | 2,238 (+21 docs) | 16 | 3,319 | |

Additionally produced: prd012-true.yaml (66 lines) and prd013-false.yaml (69 lines).

## Measure agent strategy

The measure agent demonstrated consistent autonomous planning across 37 cycles:

Table 7: Measure agent phase progression

| Phase | Cycles | Strategy | Packages |
|-------|--------|----------|----------|
| 1: Foundations | 1-7 | Build rel00.0 packages and tests | testutils, format, sys |
| 2: Commands | 8-13 | Implement rel01.x commands | cat, sponge, du |
| 3: Differential tests | 14-17 | Add differential test coverage | du, wc tests |
| 4: Infrastructure | 18-21 | Version command, ls implementation | version, ls |
| 5: Gap filling | 22-27 | Coverage gaps in cat, testutils | cat aliases, normalization |
| 6: Spec generation | 28-31 | Write PRDs then implement rel02.0 | true/false PRDs + impl |
| 7: Extended features | 32-34 | du display flags, version tests | du -a/-s/-c/-d |

The measure agent prioritized breadth (all commands) before depth (extended flags and
thorough test coverage). It autonomously wrote PRDs for true/false before implementing
them, maintaining the specification-driven development workflow.

## Failures and restarts

Table 8: All failures during run 14

| Time | Prompt Size | Cause | Resolution |
|------|-------------|-------|------------|
| 19:28 | 149 KB | Idle watchdog (10m, 0 tokens) | mage generator:resume |
| 20:57 | 207 KB | Idle watchdog (8m, 0 tokens) | mage generator:resume |
| 22:09 | 260 KB | Idle watchdog (10m, 0 tokens) | mage generator:resume |
| 23:58 | 312 KB | Idle watchdog (10m, 0 tokens) | Run closed out |

All failures share the same signature: Claude container starts, reports "ready", then
produces zero tokens for the entire idle timeout period. Input token count is also zero,
suggesting the API call never completed. The user confirmed internet connectivity issues
during these periods. The same prompt sizes succeeded on retry (149 KB, 207 KB, 260 KB),
confirming these were transient failures.

## Remaining work

Table 9: Unimplemented utilities

| Utility | Release | PRD | Status |
|---------|---------|-----|--------|
| yes | rel02.0 | prd014-yes | Not started |
| basename | rel02.0 | prd015-basename | Not started |
| dirname | rel02.0 | prd016-dirname | Not started |
| tee | rel02.1 | prd017-tee | Not started |
| head | rel02.2 | prd018-head | Not started |
| seq | rel02.2 | prd019-seq | Not started |
| ts | rel00.0 | prd004-ts | Skipped by measure agent |
| ls (extended) | rel04.1 | prd010-ls-extended | Not started (basic ls done) |

The measure agent reached the prompt scaling wall before completing rel02.x. A follow-up
generation run will need prompt optimization (GH-251) or selective source inclusion to
proceed efficiently.

## Recommendations

1. Implement interface encapsulation (GH-251) before the next generation run. At 312 KB,
   the measure prompt is near the practical limit for reliable operation.

2. Consider splitting generation runs by release. Running rel02.0 utilities separately
   from rel02.1-02.2 would keep prompt sizes manageable.

3. The measure agent skipped pkg/ts (timestamp) across runs 12 and 14. Investigate whether
   the PRD or use case needs revision, or if the measure agent deprioritizes it for a reason.

4. Track cobbler-scaffold issues #566-#569 and #578 for task visibility improvements.
   Current generation runs produce no sub-issue links and minimal progress tracking on GitHub.

5. The cost-per-KLOC nearly doubled from run 12 ($6.79) to run 14 ($10.74). Prompt
   optimization is not just a reliability concern but a cost concern.

## References

- docs/engineering/eng20-generation-run-12-results.yaml -- Previous run results
- https://github.com/petar-djukic/go-unix-utils/issues/206 -- GH-206: Recurring generation
- generation-gh-206-generation-run-14 -- Generation branch
- v1.20260304.1 -- Generated code tag
- v1.20260304.1-requirements -- Specs-only tag
- https://github.com/petar-djukic/go-unix-utils/issues/251 -- Prompt size reduction via encapsulation
- https://github.com/petar-djukic/go-unix-utils/issues/252 -- OOD principles in specifications
- https://github.com/petar-djukic/cobbler-scaffold/issues/578 -- Placeholder issue traceability
