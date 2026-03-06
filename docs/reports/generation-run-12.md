# GH-168 Generation Run 12: Full rel00.0 through rel02.2 with Idle Watchdog Fix

We ran the cobbler pipeline on generation branch generation-gh-168-generation-run-12 targeting
all releases (rel00.0 through rel02.2) with unlimited cycles. The run stitched 23 tasks across
27 cycles, producing 2,300 production LOC and 3,003 test LOC across 35 source files (5,303
total Go LOC). The run also produced 2 new PRDs (prd020-true, prd021-false) for rel02.0
trivial utilities.

This is the first run on scaffold v0.20260303.0 using Claude Opus 4.6 in podman containers.
The run revealed a critical idle watchdog issue: the Opus model needs 3-4 minutes of thinking
time before producing the first output token on 70-100KB measure prompts. The default 60-second
idle timeout killed the first two measure attempts. Increasing idle_timeout_seconds to 600
resolved the issue.

The run completed with zero stitch failures after the watchdog fix. The measure agent
demonstrated autonomous end-to-end SDD: it wrote PRDs for rel02.0 trivial utilities (true,
false) before implementing them, proving the pipeline can generate specifications and
implementations in a single generation run.

The run stopped after 27 cycles because the final measure proposed cmd/false but gh issue
create failed (likely GitHub API rate limit on issue creation). With no open issues remaining,
the generator stopped cleanly.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Generation branch | generation-gh-168-generation-run-12 |
| Scaffold version | v0.20260303.0 |
| Model | Claude Opus 4.6 (in podman containers) |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 1 |
| Max stitch issues per cycle | 1 |
| CLAUDE_CODE_MAX_OUTPUT_TOKENS | 128,000 |
| max_requirements_per_task | 4 |
| idle_timeout_seconds | 600 (increased from default 60) |
| enforce_measure_validation | false |
| max_measure_retries | 0 |
| releases | ["00.0", "01.0", "01.1", "01.2", "01.3", "02.0", "02.1", "02.2"] |
| context_include | VISION.yaml, ARCHITECTURE.yaml |
| context_exclude | SPECIFICATIONS.yaml, road-map.yaml |
| Seeded LOC | 0 (clean start) |
| Budgeted cycles | unlimited (cycles: 0) |
| Actual cycles | 27 |

## Idle watchdog fix

The first critical finding was that the idle watchdog timeout (default 60s) is incompatible
with Claude Opus on large prompts. The Opus model needs significant thinking time before
producing its first output token.

Table 2: Idle watchdog failures and resolution

| Attempt | Timeout | Prompt Size | Duration Before Kill | Tokens Out |
|---------|---------|-------------|---------------------|-----------|
| 1 | 60s | 70 KB | 1m1s | 0 |
| 2 | 180s | 70 KB | 3m1s | 0 |
| Direct test | none | 70 KB | 3m35s | 10,521 |
| 3 (fix) | 600s | 70 KB | 4m21s | 15,118 |

The direct test piped the measure prompt through podman without the watchdog, confirming
the prompt works but needs ~215 seconds before the first output token. Setting
idle_timeout_seconds to 600 resolved the issue permanently. All subsequent measure and
stitch sessions completed without watchdog interference.

Note: the scaffold documentation says "Set to 0 to disable" but the code overrides 0 to 60
(config.go defaults function). A large value (600) is the correct workaround.

## Cycle-by-cycle results

Table 3: All tasks in execution order

| # | Issue | Description | Type | +Prod | +Test | Duration | Cost |
|---|-------|-------------|------|-------|-------|----------|------|
| 1 | #183 | pkg/testutils DiffTest harness | prod | +159 | — | 2m20s | $0.68 |
| 2 | #184 | pkg/testutils unit tests | test | +38 | +304 | 11m15s | $2.06 |
| 3 | #185 | pkg/sys file metadata, signals, terminal | prod | +220 | — | 4m33s | $1.03 |
| 4 | #186 | pkg/format size formatting, columns | prod | +158 | — | 4m52s | $1.06 |
| 5 | #187 | pkg/sys unit tests | test | — | +362 | 1m47s | $0.70 |
| 6 | #188 | cmd/wc implementation | prod | +410 | — | 11m27s | $2.00 |
| 7 | #189 | cmd/wc differential tests | test | — | +266 | 2m7s | $0.69 |
| 8 | #190 | cmd/cat implementation | prod | +276 | — | 7m54s | $1.54 |
| 9 | #191 | cmd/cat differential tests | test | — | +412 | 1m41s | $0.66 |
| 10 | #192 | pkg/format R1/R3 unit tests | test | — | +422 | 4m3s | $1.02 |
| 11 | #193 | pkg/format ANSI color (R2) | prod | +132 | — | 1m53s | $1.00 |
| 12 | #194 | cmd/sponge implementation | prod | +381 | — | 8m46s | $1.87 |
| 13 | #195 | cmd/du implementation | prod | +379 | — | 12m5s | $2.86 |
| 14 | #196 | cmd/sponge differential tests | test | — | +293 | 2m23s | $0.87 |
| 15 | #197 | cmd/du differential tests | test | — | +421 | 2m3s | $0.73 |
| 16 | #198 | pkg/format color unit tests | test | — | +217 | 48s | $0.41 |
| 17 | #199 | cmd/version implementation | prod | +70 | — | 1m34s | $0.64 |
| 18 | #200 | cmd/version tests | test | — | +155 | 47s | $0.39 |
| 19 | #201 | pkg/testutils differential harness | test | — | — | 1m1s | $0.75 |
| 20 | #202 | prd020-true PRD | doc | — | — | 1m23s | $0.52 |
| 21 | #203 | prd021-false PRD | doc | — | — | 2m3s | $1.28 |
| 22 | #204 | cmd/true implementation | prod | +77 | — | 57s | $0.58 |
| 23 | #205 | cmd/true differential tests | test | — | +151 | 41s | $0.36 |

LOC totals: 2,300 prod + 3,003 test = 5,303. All stitch sessions succeeded. Zero failures
after the idle watchdog fix.

## Measure agent task ordering strategy

The measure agent demonstrated sophisticated autonomous planning. Rather than following a
strict release order, it built foundations first, then commands, then circled back for tests
and gaps.

Table 4: Measure agent phase progression

| Phase | Cycles | Strategy | Tasks |
|-------|--------|----------|-------|
| 1: Foundations | 1-5 | Build all rel00.0 production packages | testutils, sys, format |
| 2: Commands | 6-13 | Implement rel01.x commands, interleave tests | wc, cat, sponge, du |
| 3: Gap filling | 14-18 | Circle back for missing tests | format tests, color tests |
| 4: Infrastructure | 17-19 | Version command, testutils harness | version, testutils |
| 5: Spec generation | 20-23 | Write PRDs then implement rel02.0 | true PRD, false PRD, true impl |

The measure agent also skipped pkg/ts (timestamp) and pkg/format tests initially, built all
four commands, then returned to fill coverage gaps. This maximized functional output early
while deferring non-blocking work.

## Measure prompt growth

The measure prompt includes all Go source files in pkg/ and cmd/, causing it to grow
linearly as more code is generated.

Table 5: Measure prompt size over cycles

| Cycle | Prompt Size | Source Files | Prod LOC | Cost |
|-------|-------------|-------------|----------|------|
| 1 | 70 KB | 0 | 0 | $0.40 |
| 3 | 93 KB | 9 | 197 | $0.54 |
| 5 | 103 KB | 9 | 417 | $0.58 |
| 7 | 126 KB | 15 | 575 | $0.58 |
| 9 | 152 KB | — | 985 | $0.67 |
| 11 | 179 KB | — | 1,261 | $1.09 |
| 13 | 217 KB | — | 1,774 | $0.84 |
| 15 | 262 KB | — | 2,153 | $0.78 |
| 17 | 279 KB | — | 2,223 | $0.81 |
| 27 | 282 KB | — | 2,300 | $0.69 |

Prompt size grew from 70 KB (0 source files) to 282 KB (35+ source files) over 27 cycles.
The growth is approximately 7-10 KB per stitch cycle. At the end of the run, the measure
prompt consumed ~108K input tokens. For larger codebases, the measure prompt could exceed
model context limits. Consider adding a context budget that caps included source files or
uses summarization.

## Cost analysis

Table 6: Cost breakdown

| Category | Cost |
|----------|------|
| Stitch (23 tasks) | ~$22 |
| Measure (27 cycles, 2 failed) | ~$14 |
| Total | ~$36 |

At ~$36 for 5,303 Go LOC, the cost is ~$6.79 per 1,000 LOC. This is higher than run 11
(~$5.46/KLOC with Sonnet) because Opus tokens are more expensive. However, the zero-failure
rate and autonomous PRD generation (tasks #202, #203) offset the higher per-token cost.

Table 7: Cost per LOC comparison across runs

| Run | Model | Total LOC | Est. Cost | Cost/KLOC |
|-----|-------|-----------|-----------|-----------|
| 11 (eng18) | Sonnet 4.6 | 4,218 | ~$23 | ~$5.46 |
| 12 (eng20) | Opus 4.6 | 5,303 | ~$36 | ~$6.79 |

## Generated packages

Table 8: All packages produced in run 12

| Package | Prod Files | Prod LOC | Test Files | Test LOC | Release |
|---------|-----------|----------|-----------|----------|---------|
| pkg/testutils | 1 | 159 | 2 | 304 | rel00.0 |
| pkg/sys | 5 | 220 | 3 | 362 | rel00.0 |
| pkg/format | 4 | 290 | 3 | 639 | rel00.0 |
| cmd/version | 1 | 70 | 1 | 155 | rel00.0 |
| cmd/wc | 1 | 410 | 1 | 266 | rel01.0 |
| cmd/cat | 1 | 276 | 1 | 412 | rel01.1 |
| cmd/sponge | 1 | 381 | 1 | 293 | rel01.2 |
| cmd/du | 1 | 379 | 1 | 421 | rel01.3 |
| cmd/true | 1 | 77 | 1 | 151 | rel02.0 |
| testdata | 1 | 38 | — | — | rel00.0 |
| Total | 17 | 2,300 | 14 | 3,003 | |

Additionally produced: prd020-true.yaml (65 lines) and prd021-false.yaml (65 lines).

## Comparison with run 11

Table 9: Cross-run comparison

| Metric | Run 11 (eng18) | Run 12 (eng20) |
|--------|----------------|----------------|
| Scaffold version | v0.20260302.0 | v0.20260303.0 |
| Model | Sonnet 4.6 | Opus 4.6 |
| Target releases | rel00.0-01.3 | rel00.0-02.2 |
| Cycles | ~14 | 27 |
| Tasks stitched | 13 | 23 |
| Success rate | 81% (3 Claude timeouts) | 100% (after watchdog fix) |
| Prod LOC | 2,073 | 2,300 |
| Test LOC | 2,145 | 3,003 |
| Total LOC | 4,218 | 5,303 |
| PRDs generated | 0 | 2 |
| Files created | 24 | 35 |
| Estimated cost | ~$23 | ~$36 |
| Cost/KLOC | ~$5.46 | ~$6.79 |
| Runtime | ~4 hours | ~3.5 hours |
| Dominant failure | Claude service instability | Idle watchdog (fixed) |

Run 12 produced 25% more LOC, had 100% task success rate, wrote PRDs autonomously, and
completed faster despite targeting more releases. The Opus model is more expensive per token
but required fewer turns per task (no rate-limit pauses observed).

## Remaining work

The run did not complete all rel02.x utilities. The following remain unimplemented:

Table 10: Remaining work

| Utility | Release | Status |
|---------|---------|--------|
| false | rel02.0 | PRD written, implementation pending (gh issue create failed) |
| basename | rel02.0 | PRD exists (prd015), not yet proposed by measure |
| dirname | rel02.0 | PRD exists (prd016), not yet proposed by measure |
| ls (basic) | rel02.0 | PRD exists (prd008), not yet proposed |
| ls (extended) | rel02.0 | PRD exists (prd010), not yet proposed |
| tee | rel02.1 | PRD exists (prd017), not yet proposed |
| head | rel02.2 | PRD exists (prd018), not yet proposed |
| seq | rel02.2 | PRD exists (prd019), not yet proposed |
| pkg/ts | rel00.0 | PRD exists (prd004), skipped by measure agent |

The measure agent prioritized foundations and rel01.x commands over rel02.x utilities.
A follow-up generation run should complete the remaining utilities.

## Known issues

1. Idle watchdog incompatible with Opus thinking time. The scaffold default of 60s kills
   Opus measure sessions that need 3-4 minutes before first output. Workaround:
   idle_timeout_seconds: 600 in configuration.yaml.

2. generator:stop merges into main regardless of current branch. The close-out procedure
   requires creating a feature branch first, but generator:stop's ensureOnBranch function
   switches to the generation branch then merges into main. Workaround: manually move
   the work to a feature branch and reset main after generator:stop.

3. gh issue create failed on the final measure cycle, preventing cmd/false issue creation.
   Likely a GitHub API rate limit after 27 cycles of issue creation/closure.

4. Measure prompt grows linearly with source code. At 282 KB for 2,300 prod LOC, scaling
   to 10K+ LOC would push the prompt near context limits. A context budget or
   summarization strategy is needed for larger codebases.

## Recommendations

1. Set idle_timeout_seconds to 600 as the default for Opus in cobbler-scaffold. The current
   default of 60s is incompatible with Opus thinking time.

2. Fix generator:stop to respect the current branch. It should merge into whatever branch
   is checked out, not force-switch to main.

3. Add a context budget to the measure prompt. Cap total source file inclusion at a
   configurable byte limit (e.g., 200 KB) and summarize or exclude older files.

4. Run a follow-up generation (run 13) targeting the remaining rel02.x utilities. The
   foundations and rel01.x are complete, so the run should focus on basename, dirname,
   false, ls, tee, head, and seq.

5. Investigate the gh issue create failure. If it was a rate limit, add retry logic with
   exponential backoff to the issue creation step.

## References

- docs/engineering/eng18-generation-run-11-results.yaml -- Previous run results
- https://github.com/petar-djukic/go-unix-utils/issues/168 -- GH-168: Recurring generation
- generation-gh-168-generation-run-12 -- Generation branch
- v1.20260303.1 -- Generated code tag
- v1.20260303.1-requirements -- Specs-only tag
