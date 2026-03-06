# GH-147 Generation Run 11: Full rel00.0 through rel01.3 with Claude Service Instability

We ran the cobbler pipeline on generation branch generation-2026-03-02-17-54-58 targeting
all releases (rel00.0 through rel01.3) with 20 cycles budgeted. The run stitched 13 tasks,
producing 2,073 production LOC and 2,145 test LOC across 24 source files (4,218 total LOC).
The run completed in two phases due to Claude service instability that caused three
consecutive stitch timeouts for the cmd/wc task.

This is the first run on cobbler-scaffold v0.20260302.0, which fixed the parseIssuesJSON
bug that caused duplicate task proposals in run 10 (eng17). The fix worked: no duplicate
tasks were proposed and all 13 tasks were unique. The max_measure_issues=1 setting also
worked correctly throughout.

The dominant failure mode was Claude API service degradation. Three consecutive stitch
attempts for cmd/wc (#158) timed out with 0 tokens over 45 minutes total. The generator
was stopped, the service recovered, and the run resumed successfully. The fourth attempt
for #158 completed in 14m49s.

Compared to run 10 (eng17), this run produced fewer total LOC (4,218 vs 5,128) because
run 10's version package was larger (415 LOC testutils vs 278 LOC) and run 10 included
duplicate tasks that inflated the stitch count. The actual unique production output is
comparable. Task ordering was deterministic across runs 10 and 11: both started with
pkg/format, then pkg/sys, then pkg/testutils, confirming that the scaffold v0.20260302.0
ordering is stable.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Spec baseline tag | v0.20260302.2 |
| Generation branch | generation-2026-03-02-17-54-58 |
| Scaffold version | v0.20260302.0 |
| Model | Claude Sonnet 4.6 |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 1 |
| Max stitch issues per cycle | 1 |
| CLAUDE_CODE_MAX_OUTPUT_TOKENS | 128,000 |
| max_requirements_per_task | 4 |
| enforce_measure_validation | false |
| max_measure_retries | 0 |
| releases | ["00.0", "01.0", "01.1", "01.2", "01.3"] |
| context_include | VISION.yaml, ARCHITECTURE.yaml |
| context_exclude | SPECIFICATIONS.yaml, road-map.yaml |
| Seeded LOC | 0 (clean start) |
| Budgeted cycles | 20 |
| Actual cycles | ~14 (13 tasks + 1 empty measure) |

## Timeline

The run split into two phases due to Claude service instability.

Table 2: Phase timeline

| Phase | Time (EST) | Activity |
|-------|-----------|----------|
| 1 start | 17:55 | generator:run 20 started |
| 1 measure | 17:55-17:59 | First measure proposed #153 (pkg/format) |
| 1 stitch | 17:59-18:20 | #153 stitched (10min rate limit pause mid-session) |
| 1 stitch | 18:20-19:05 | #155, #156, #157 stitched (stats not captured) |
| 1 stitch | 19:05-19:20 | #158 attempt 1: 15m timeout, 0 tokens |
| 1 stitch | 19:23-19:38 | #158 attempt 2: 15m timeout, 0 tokens |
| 1 stitch | 19:41-19:56 | #158 attempt 3: 15m timeout, 0 tokens |
| 1 stop | 19:56 | Generator stopped manually |
| Gap | 19:56-20:40 | Waited for Claude service recovery |
| 2 start | 20:40 | generator:run 20 resumed |
| 2 stitch | 20:40-20:55 | #158 attempt 4: success (14m49s, $2.79) |
| 2 stitch | 20:55-21:30 | #159, #160, #161, #162 stitched |
| 2 stitch | 21:31-21:56 | #163, #164, #165, #166 stitched |
| 2 measure | 22:02-22:06 | Final measure proposed 0 issues |
| End | 22:06 | Generation complete |

Total wall-clock time: ~4 hours (17:55-22:06), of which ~45 min was wasted on
service-degradation timeouts and ~44 min on the gap between phases.

## Cycle-by-cycle results

Table 3: All tasks in execution order

| Order | Issue | Description | Files | +Prod | +Test | Duration | Cost | Status |
|-------|-------|-------------|-------|-------|-------|----------|------|--------|
| 1 | #153 | pkg/format R1-R3 | 6 | +295 | +391 | ~11m | ~$1.50 | success |
| 2 | #155 | pkg/sys R1-R4 | 9 | +293 | +191 | ~5m | ~$1.20 | success |
| 3 | #156 | pkg/testutils R1-R4 | 1 | +278 | — | ~3m | ~$0.80 | success |
| 4 | #157 | cmd/version | 2 | +54 | +109 | ~3m | ~$0.80 | success |
| 5a | #158 | cmd/wc (attempt 1) | 0 | — | — | 15m0s | $0.00 | timeout |
| 5b | #158 | cmd/wc (attempt 2) | 0 | — | — | 15m0s | $0.00 | timeout |
| 5c | #158 | cmd/wc (attempt 3) | 0 | — | — | 15m0s | $0.00 | timeout |
| 5d | #158 | cmd/wc (attempt 4) | 2 | +480 | — | 14m49s | $2.79 | success |
| 6 | #159 | cmd/cat R1-R4 | 2 | +273 | — | ~6m | ~$1.40 | success |
| 7 | #160 | cmd/sponge R1-R4 | 2 | +82 | — | ~3m | ~$0.80 | success |
| 8 | #161 | cmd/du R1-R4 | 2 | +318 | — | ~14m | ~$2.50 | success |
| 9 | #162 | testutils tests R1-R5 | 1 | — | +175 | ~5m | ~$1.00 | success |
| 10 | #163 | wc tests R1-R6 | 1 | — | +255 | ~7m | ~$1.20 | success |
| 11 | #164 | cat tests R1-R5 | 1 | — | +402 | ~6m | ~$1.20 | success |
| 12 | #165 | sponge tests R1-R4 | 2 | — | +306 | ~5m | ~$1.00 | success |
| 13 | #166 | du tests R1-R4 | 2 | — | +316 | ~6m | ~$1.20 | success |

LOC totals: 2,073 prod + 2,145 test = 4,218. Tilde (~) values are estimates where
exact stats were not captured; the orchestrator cleaned history files between cycles.

## Confirmed stitch session metrics

The orchestrator history directory was cleaned between cycles, so detailed token-level
stats were only captured for sessions observed during monitoring. The cmd/wc (#158)
session is the most thoroughly documented.

Table 4: Stitch #158 cmd/wc — all four attempts

| Attempt | Duration | Input Tokens | Output Tokens | Cache Create | Cache Read | Cost | Turns | Outcome |
|---------|----------|-------------|---------------|-------------|------------|------|-------|---------|
| 1 | 15m0s | 0 | 0 | 0 | 0 | $0.00 | 0 | timeout |
| 2 | 15m0s | 0 | 0 | 0 | 0 | $0.00 | 0 | timeout |
| 3 | 15m0s | 0 | 0 | 0 | 0 | $0.00 | 0 | timeout |
| 4 | 14m49s | 2,067,951 | 48,359 | 93,764 | 1,974,162 | $2.79 | 44 | success |

Attempts 1-3 all showed 0 tokens, meaning Claude never responded. This was Claude
service instability, not rate limiting — the user confirmed they were nowhere near
their API usage limits and that "claude has been going up and down today."

Table 5: Confirmed measure session metrics

| Session | Duration | Input Tokens | Output Tokens | Cache Create | Cache Read | Cost | Issues Proposed |
|---------|----------|-------------|---------------|-------------|------------|------|----------------|
| M1 (initial) | 3m55s | 34,597 | 13,428 | 18,600 | 15,995 | $0.46 | 1 (#153) |
| M2 (after 4 tasks) | 2m53s | 60,332 | 10,021 | 41,107 | 19,223 | $0.52 | 1 (#158) |
| M3 (after wc fail) | 2m59s | 60,390 | 10,261 | 41,165 | 19,223 | $0.52 | 1 (#161) |
| M-final | 4m15s | 97,560 | 14,001 | 78,266 | 19,292 | $0.85 | 0 (complete) |

Measure input tokens grew from 35K to 98K as more source files were added to the
prompt. The final measure included 25 source files (252KB prompt), yet still completed
in 4m15s.

## Cost estimate

Table 6: Estimated total generation cost

| Category | Confirmed | Estimated |
|----------|-----------|-----------|
| Measures (4 confirmed + ~10 uncaptured) | $2.35 | $7-8 |
| Stitch #158 (4 attempts) | $2.79 | $2.79 |
| Stitch #153 (pkg/format, rate-limited) | — | $1.50 |
| Stitch #155-157 (3 infra tasks) | — | $2.80 |
| Stitch #159-160 (cat, sponge) | — | $2.20 |
| Stitch #161 (du, ~14m with rate limit) | — | $2.50 |
| Stitch #162-166 (5 test tasks) | — | $5.60 |
| Total | $5.14 confirmed | ~$22-25 |

At ~$23 for 4,218 LOC, the cost is ~$5.46 per 1,000 LOC. This is higher than run 10
($18 estimated for 5,128 LOC = $3.51/KLOC) due to the three wasted timeout cycles
consuming 45 minutes but no tokens, and the long rate-limit pauses in #158 and #161
that drove up cache_read costs.

## Claude service instability analysis

The dominant failure mode in this run was Claude API service degradation. Three
consecutive 15-minute stitch attempts for #158 produced zero tokens.

Table 7: Service degradation timeline

| Time (EST) | Event | Tokens |
|-----------|-------|--------|
| 19:05 | #158 attempt 1 starts | 0 over 15 min |
| 19:20 | Attempt 1 times out | |
| 19:23 | #158 attempt 2 starts | read files in 22s, then silent 14+ min |
| 19:38 | Attempt 2 times out | |
| 19:41 | #158 attempt 3 starts | same pattern as attempt 2 |
| 19:56 | Attempt 3 times out, generator stopped | |
| 20:40 | #158 attempt 4 starts (after recovery) | 2M input, 48K output |
| 20:55 | Attempt 4 succeeds | |

Attempt 2 is instructive: Claude read the required files in the first 22 seconds
(turns 1-8), then sat "waiting for LLM" for 14 minutes before timing out. This
confirms the service accepted the request but failed to generate a response.

The service instability also caused rate-limit pauses during successful stitches:
10m12s pause during #158 attempt 4 (turns 15-16), 10m21s pause during #161 (turns
23-24), and a ~10m pause during #153 (turn 5). These pauses are labeled as rate
limits by the orchestrator but may actually be service throttling during degradation.

## Files Claude reads beyond required_reading

The user requested analysis of what files Claude reads during stitch sessions beyond
the files specified in the issue's required_reading list. We captured file read
patterns from three observed stitch sessions.

Table 8: Extra file reads by stitch session

| Session | Required Reading Files | Extra Files Read | Purpose |
|---------|----------------------|-----------------|---------|
| #158 cmd/wc | prd005-wc.yaml, uc001-wc.yaml, test-rel-01.0.yaml, difftest.go, version/main.go, ARCHITECTURE.yaml | go.mod, existing wc binary, ls of worktree | Module path, existing state check |
| #161 cmd/du | prd008-du.yaml, uc001-du.yaml, test-rel-01.3.yaml, stat.go, size.go, version/main.go | go.mod, cmd/wc/main.go, cmd/cat/main.go, cmd/sponge/main.go, pkg/sys/signal.go | Cross-referencing 4 existing implementations for patterns |
| #166 du tests | prd009-du.yaml, uc001-du.yaml, test-rel-01.3.yaml, du/main.go, difftest.go | cmd/wc/wc_test.go, cmd/sponge/sponge_test.go | Cross-referencing test patterns |

Patterns observed:

1. cmd/version/main.go is consistently read as a pattern reference for the
   testable-main run() function signature, even when it is already in required_reading.

2. Later stitch sessions read all previously-generated cmd/ implementations. The
   #161 (du) session read wc, cat, and sponge source files before writing du. This
   cross-referencing produces more consistent code style across commands but adds
   ~15 tool calls and increases cache_read token counts substantially.

3. go.mod is read in nearly every session, even though the module path is provided
   in the issue body. This is a defensive read that costs minimal tokens.

4. Test stitch sessions cross-reference 2-3 existing test files to match patterns.
   This is beneficial for consistency but means test tasks later in the queue are
   heavier on input tokens.

The extra reads are beneficial — they produce consistent code across commands. The
cost is higher cache_read token counts (1.97M for #158's successful attempt), but
cache reads are the cheapest token category. No intervention is needed.

## Long stitch analysis and requirement splitting

The user requested analysis of whether long stitch runs indicate requirements that
should be split further.

Table 9: Stitch duration vs requirement count

| Issue | Description | Reqs | Sub-reqs | +LOC | Duration | Cost |
|-------|-------------|------|----------|------|----------|------|
| #153 | pkg/format | 3 | 3 | 686 | ~11m | ~$1.50 |
| #155 | pkg/sys | 4 | 4 | 488 | ~5m | ~$1.20 |
| #156 | pkg/testutils | 4 | 4 | 278 | ~3m | ~$0.80 |
| #157 | cmd/version | 4 | 4 | 163 | ~3m | ~$0.80 |
| #158 | cmd/wc | 4 | 4 | 480 | 14m49s | $2.79 |
| #159 | cmd/cat | 4 | 4 | 273 | ~6m | ~$1.40 |
| #160 | cmd/sponge | 4 | 4 | 82 | ~3m | ~$0.80 |
| #161 | cmd/du | 4 | 4 | 318 | ~14m | ~$2.50 |
| #162 | testutils tests | 4 | 4 | 175 | ~5m | ~$1.00 |
| #163 | wc tests | 4+ | 6 | 255 | ~7m | ~$1.20 |
| #164 | cat tests | 4 | 4 | 402 | ~6m | ~$1.20 |
| #165 | sponge tests | 4 | 4 | 306 | ~5m | ~$1.00 |
| #166 | du tests | 4 | 4 | 316 | ~6m | ~$1.20 |

Observations:

1. All implementation tasks had exactly 4 top-level requirements (the
   max_requirements_per_task=4 cap was effective). The actual complexity varies:
   cmd/wc R1-R4 each expand to multi-part behaviors (flags, formatting, stdin
   handling, error codes), while cmd/sponge R1-R4 are each self-contained.

2. The two longest stitches were #158 (cmd/wc, 14m49s) and #161 (cmd/du, ~14m).
   Both are the most complex commands in the set: wc has 6 counting modes, column
   alignment, and multi-file totals; du has recursive traversal, hard-link dedup,
   6 flags, and depth limiting. Both also experienced 10-minute rate-limit pauses
   mid-session, inflating their wall-clock time.

3. Excluding rate-limit pauses, the actual coding time for #158 was ~4m37s (44
   turns minus 10m12s pause). For #161, coding time was ~3m40s (37+ turns minus
   10m21s pause). These are comparable to the 3-6 minute range of simpler tasks.

4. LOC output does not strongly predict duration. #153 (pkg/format) produced 686 LOC
   in ~11m, while #164 (cat tests) produced 402 LOC in ~6m. The variance comes
   from rate-limit pauses, not requirement complexity.

Conclusion: The max_requirements_per_task=4 cap adequately bounds task complexity.
The long stitch durations in #158 and #161 were caused by rate-limit pauses, not by
excessive requirements. Splitting these tasks further would not reduce their actual
coding time (4-5 minutes when excluding pauses) and would add overhead from
additional measure cycles. No requirement splitting changes are recommended.

## Deterministic ordering confirmation

Runs 10 and 11 both used scaffold v0.20260302.0 with identical configuration. The
task ordering was identical for the first four tasks:

Table 10: Task ordering comparison

| Order | Run 10 (eng17) | Run 11 (eng18) |
|-------|---------------|----------------|
| 1 | pkg/format (#98) | pkg/format (#153) |
| 2 | pkg/sys (#99) | pkg/sys (#155) |
| 3 | pkg/testutils (#100) | pkg/testutils (#156) |
| 4 | cmd/version (#104) | cmd/version (#157) |
| 5 | pkg/testutils tests (#101) | cmd/wc (#158) |
| 6 | pkg/format tests (#102) | cmd/cat (#159) |
| 7 | pkg/sys tests (#103) | cmd/sponge (#160) |
| 8 | cmd/version tests (#105) | cmd/du (#161) |

The ordering diverges at position 5 because run 10 used a two-phase approach
(rel00.0 first, then rel01.x), while run 11 had all releases enabled from the start.
The measure agent chose to start command implementations before writing rel00.0 test
files. This is an acceptable ordering since the commands depend on the packages but
not on the package test files.

The first four positions (format → sys → testutils → version) are deterministic
across both runs, confirming that the scaffold's ordering logic produces stable results
for the infrastructure layer.

## Generated packages

Table 11: All packages produced in run 11

| Package | Prod Files | Prod LOC | Test Files | Test LOC | Release |
|---------|-----------|----------|-----------|----------|---------|
| pkg/format | 3 | 295 | 3 | 391 | rel00.0 |
| pkg/sys | 5 | 293 | 1 | 191 | rel00.0 |
| pkg/testutils | 1 | 278 | 1 | 175 | rel00.0 |
| cmd/version | 1 | 54 | 1 | 109 | rel00.0 |
| cmd/wc | 1 | 480 | 1 | 255 | rel01.0 |
| cmd/cat | 1 | 273 | 1 | 402 | rel01.1 |
| cmd/sponge | 1 | 82 | 1 | 306 | rel01.2 |
| cmd/du | 1 | 318 | 1 | 316 | rel01.3 |
| Total | 14 | 2,073 | 10 | 2,145 | |

## Cross-run comparison

Table 12: Comparison across recent generation runs

| Metric | eng16 (run 9) | eng17 (run 10) | eng18 (run 11) |
|--------|---------------|----------------|----------------|
| Spec baseline | v0.20260301.0 | v0.20260301.0 | v0.20260302.2 |
| Scaffold version | v0.20260301.2 | v0.20260301.2 | v0.20260302.0 |
| Seeded LOC | 0 | 0 | 0 |
| Target releases | per-release | multi-release | all releases |
| Budgeted cycles | 10 | 21 (9+12) | 20 |
| Actual cycles | 10 | 21 | ~14 |
| Tasks stitched | 9/10 | 18/21 (16 unique) | 13/16 (13 unique) |
| Success rate | 90% | 86% (76% unique) | 81% (100% unique) |
| Prod LOC | 864 | 2,174 | 2,073 |
| Test LOC | 1,464 | 2,954 | 2,145 |
| Total LOC | 2,328 | 5,128 | 4,218 |
| Files created | 17 | 30 | 24 |
| Dominant failure | cycle limit | issues JSON parsing | Claude service instability |
| Estimated cost | — | ~$18 | ~$23 |
| Cost per KLOC | — | ~$3.51 | ~$5.46 |

Run 11 has 100% unique task success rate (all 13 tasks unique, all 13 succeeded) — the
highest unique success rate of any multi-release run. The 81% raw rate includes the 3
timeout attempts caused by Claude service instability, which were transient infrastructure
failures outside the pipeline's control.

## Known issues

1. Binary artifact leak: cat, du, sponge, and wc binaries (ELF aarch64, 2.3-2.7MB each)
   were committed to the generation branch. The cleanGoBinaries function reported
   "removed 0 binary file(s)". Tracked as cobbler-scaffold #456.

2. Stitch history cleanup: The orchestrator deletes .cobbler/history/ between cycles,
   making it impossible to collect per-session stats after the fact. Consider preserving
   stats files outside the history directory or appending to a cumulative log.

3. Rate-limit pauses during successful stitches: 10-minute pauses occurred in #153,
   #158, and #161. The claudeAiOauth credentials share the user's interactive rate
   limit pool. Dedicated API credentials would avoid contention.

## Recommendations

1. Preserve stitch stats across cycles. The orchestrator should write each session's
   token counts, cost, and duration to a cumulative file (e.g., .cobbler/stats-log.yaml)
   that survives history cleanup. This would eliminate the data gaps in this report.

2. Add a health check timeout. When a stitch session produces 0 tokens for 60 seconds,
   it should be terminated early rather than waiting the full 15 minutes. Three 15-minute
   timeouts with 0 tokens wasted 45 minutes in this run.

3. The max_requirements_per_task=4 cap is working well. No task exceeded 4 top-level
   requirements and all completed within the 15-minute budget (excluding rate-limit
   pauses). Do not reduce this cap.

4. Consider separating stitch API credentials from the user's interactive session to
   avoid rate-limit contention that causes mid-session pauses.

5. Fix binary leak (cobbler-scaffold #456). The .gitignore should exclude compiled
   binaries from the generation branch, or the stitch post-commit hook should clean
   them.

## References

- docs/engineering/eng17-generation-run-10-results.yaml -- Previous run results
- https://github.com/petar-djukic/go-unix-utils/issues/147 -- GH-147: Recurring generation
- https://github.com/petar-djukic/go-unix-utils/issues/153 -- Task 153: pkg/format
- https://github.com/petar-djukic/go-unix-utils/issues/155 -- Task 155: pkg/sys
- https://github.com/petar-djukic/go-unix-utils/issues/156 -- Task 156: pkg/testutils
- https://github.com/petar-djukic/go-unix-utils/issues/157 -- Task 157: cmd/version
- https://github.com/petar-djukic/go-unix-utils/issues/158 -- Task 158: cmd/wc
- https://github.com/petar-djukic/go-unix-utils/issues/159 -- Task 159: cmd/cat
- https://github.com/petar-djukic/go-unix-utils/issues/160 -- Task 160: cmd/sponge
- https://github.com/petar-djukic/go-unix-utils/issues/161 -- Task 161: cmd/du
- v1.20260302.3 -- Generated code tag
- generation-2026-03-02-17-54-58 -- Generation branch
