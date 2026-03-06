# GH-29 Generation Run 6: First Clean Benchmark Attempt

We ran the cobbler pipeline as the first clean (unseeded) generation attempt on generation
branch generation-2026-02-27-21-48-32. The goal was to validate the full measure+stitch
cycle without seeding from previous v1 tags, establish a benchmark of end-to-end code
generation from specifications only, and exercise the atomicity improvements from GH-31.

The measure phase completed normally, proposing three tasks. Two tasks (cmd/wc, cmd/sponge)
completed successfully. A third task (cmd/ls -i,-s,-n,-F, prd010-ls-extended R3/R4) failed
after five stitch attempts: per-request API rate limits caused 10-15 minute LLM pauses
within each 15-minute stitch window, leaving insufficient time to complete and commit the
implementation. We also encountered a 5-hour API quota exhaustion between the measure and
stitch phases, requiring an account switch.

As a secondary outcome, we identified that the stitch agent repeatedly reinvents shared
infrastructure (pkg/testutils, pkg/sys) on each retry from scratch. We pre-committed both
packages to the generation branch (commit b8de78e) and filed GH-33 to address the root
cause via an updated prd001-testutils that serves as a stitch API reference.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Baseline tag | v0.20260227.3 |
| Generation branch | generation-2026-02-27-21-48-32 |
| Model | Claude Sonnet 4.6 |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 3 (iterative, 1 per iteration) |
| Max stitch issues per cycle | 10 |
| CLAUDE_CODE_MAX_OUTPUT_TOKENS | 128,000 |
| cobbler-scaffold version | v0.20260227.3 |
| max_requirements_per_task | 4 |
| enforce_measure_validation | false |
| max_measure_retries | 0 |
| releases filter | none (all releases) |
| context_exclude | VISION, SPECIFICATIONS, dependency-map, sources (4 files) |
| Standard files after exclusion | 23 files (from 27) |
| Seeded LOC | 0 (clean start) |
| Benchmark validity | valid — first clean unseeded run |

This is the first generation run to start from 0 Go LOC without seeding, making it the
first valid end-to-end benchmark. The releases filter was not applied so measure could
propose any task across all releases.

## Measure phase results

The measure phase ran 3 iterations across a single measure cycle.

Table 2: Measure iteration outcomes

| Iteration | Task | ID | Prompt bytes | Cost | Duration | Notes |
|-----------|------|----|-------------|------|----------|-------|
| 1 | cmd/wc word, line, byte counter (prd005-wc R1-R6) | 13w | ~261K | $0.81 | 3m56s | req count 4 warning |
| 2 | cmd/sponge soak-before-write utility (prd007-sponge R1-R5) | zep | ~261K | $0.67 | 2m48s | req count 4 warning |
| 3 | cmd/ls metadata display and classify (-i,-s,-n,-F, prd010-ls-extended R3,R4) | 21v | ~261K | $0.81 | 4m3s | scoped task (R3+R4 only) |

Total measure: ~$2.29, 10m47s.

All three tasks triggered validation warnings ("requirement count 4 outside P9 range 5-8")
but were accepted under validate-and-warn mode. Task 21v demonstrates the GH-31 atomicity
improvement: measure proposed a scoped ls-extended task covering only R3 and R4, not the
full PRD. This was the expected improvement from rewriting prd010-ls-extended to decouple
requirements by subsystem.

A 5-hour API quota exhaustion occurred between the measure phase and stitch phase (at
approximately 22:00 EST). All API tokens were consumed by the measure phase (~249K output
tokens). Stitch started looping with immediate failures (~5 seconds per attempt) before we
detected the pattern and stopped the generator. An account switch restored API access.

## Stitch phase results

Table 3: Stitch task outcomes

| Task | Description | Outcome | Stitch attempts | Notes |
|------|-------------|---------|-----------------|-------|
| 13w | cmd/wc word, line, byte counter | success | 1 | committed 00f6f35+1dcd2c9 |
| zep | cmd/sponge soak-before-write utility | success | 1 | committed 585cc9b+e38e810 |
| 21v | cmd/ls metadata display and classify (-i,-s,-n,-F) | failed | 5 | task reset to open |

Tasks 13w and zep both completed within the 15-minute stitch window on their first attempt.
Task 21v failed five times. All five failures share the same root cause: per-request rate
limits caused 10-15 minute LLM pauses within the stitch window, consuming the budget before
the agent could write, build, test, and commit the implementation.

The stitch agent reached the write phase in three of five attempts (turns 45-55), writing
pkg/testutils, pkg/sys, and cmd/ls/main.go before running out of time. The compiled output
was never committed.

## Task 21v failure analysis

Task 21v (cmd/ls -i,-s,-n,-F) failed for a compound reason:

1. Infrastructure cost. The task requires pkg/testutils (differential test harness) and
   pkg/sys (syscall wrappers for inode, blocks, UID, GID) to be implemented before cmd/ls
   itself can be written or tested. Neither package existed at the start of any attempt,
   since the worktree is reset to the generation branch HEAD on each retry. The agent
   spent 8-11 minutes per attempt writing infrastructure before reaching the cmd/ls
   implementation.

2. Per-request rate limits. Every few turns, the stitch agent received a transient rate
   limit event (rateLimitType not confirmed — distinct from the 5-hour quota). Each event
   caused a 10-15 minute LLM response pause. This pause alone exhausted the 15-minute
   budget without the agent producing a single line of cmd/ls code.

3. Combined effect. Infrastructure cost (~10 min) plus a rate limit pause (~10 min) exceeds
   the 15-minute budget. When no rate limit pause occurred, the agent completed 13 minutes
   of work and timed out with compilation passing but no test file written.

Mitigation applied this session: we committed pkg/testutils and pkg/sys to the generation
branch (commit b8de78e) before the fifth stitch attempt. The fifth attempt found both
packages already present, read them at turns 8-13 (26s elapsed), and began writing
cmd/ls/main.go at turn 14. However, a rate limit pause at turn 18 consumed the remaining
budget and the attempt ended at turn 18 without producing output.

Root cause for future runs: see GH-33 for the prd001-testutils API reference improvement.
For the rate limit issue, the stitch window must be long enough that a single rate limit
pause does not consume the entire budget. The current 15-minute window is insufficient
when rate limits add 10-15 minutes of latency.

## GH-31 atomicity validation

The measure agent correctly proposed a scoped task for ls-extended. Instead of proposing
all five requirement groups (R1-R5) as a single task, it proposed only R3 (metadata display:
-i, -s, -n) and R4 (file type classification: -F). This is direct evidence that the GH-31
PRD rewrites improved measure's scoping behavior.

The atomicity improvement is necessary but not sufficient: even a correctly scoped task can
fail the stitch budget if it requires building shared infrastructure from scratch. GH-33
addresses the infrastructure cost.

## Infrastructure commits

As a secondary outcome, we committed shared infrastructure to the generation branch that
the stitch agent had been recreating from scratch on every attempt.

Table 4: Infrastructure committed (commit b8de78e)

| Package | Files | LOC | Notes |
|---------|-------|-----|-------|
| pkg/testutils | difftest.go, normalizers.go | 163 | simplified version; branch has a fuller implementation |
| pkg/sys | sys.go, stat_darwin.go, stat_linux.go, terminal.go | 159 | simplified syscall wrappers |

These are simplified implementations written during the session, not generated by the
stitch pipeline. The generation branch's pkg/sys lacks golang.org/x/sys/unix integration
(using inline struct instead of unix.IoctlGetWinsize). The feature branch (gh-29) has a
fuller version of both packages from previous stitch runs.

GH-33 (Improve prd001-testutils as stitch API reference) addresses the root cause: if
prd001-testutils is updated to serve as a complete API reference and included in every
cmd/ task's required_reading, stitch agents will import the package rather than reinvent it.

## LOC generated

Table 5: Code generated by the stitch pipeline this run

| Package | File | LOC | Source |
|---------|------|-----|--------|
| cmd/wc | main.go | 340 | stitch agent (task 13w) |
| cmd/sponge | main.go | 300 | stitch agent (task zep) |
| pkg/testutils | difftest.go | 128 | session (not stitch) |
| pkg/testutils | normalizers.go | 35 | session (not stitch) |
| pkg/sys | sys.go | 18 | session (not stitch) |
| pkg/sys | stat_darwin.go | 55 | session (not stitch) |
| pkg/sys | stat_linux.go | 55 | session (not stitch) |
| pkg/sys | terminal.go | 31 | session (not stitch) |

Stitch-generated production LOC: 640 (cmd/wc + cmd/sponge).
Session-added infrastructure LOC: 322 (pkg/testutils + pkg/sys).
Total new production LOC on generation branch: 962.
Test LOC generated: 0 (task 21v never committed).

## Aggregate results

Table 6: Run totals

| Metric | Value |
|--------|-------|
| Measure cost | ~$2.29 |
| Stitch cost | not captured |
| Total wall clock | ~10h (21:48 start, stopped at 07:02+) |
| Productive stitch time | ~2 tasks × avg 10 min = ~20 min |
| Stitch attempts (task 21v) | 5 |
| Stitch resets (task 21v) | 4 committed, 1 aborted |
| Tasks completed | 2 of 3 |
| Tasks failed | 1 (21v, 5 attempts) |
| New stitch-generated LOC (prod) | 640 |
| New stitch-generated LOC (test) | 0 |
| 5-hour quota exhaustion | 1 (between measure and stitch, ~22:00 EST) |
| Benchmark validity | valid (clean start) |

## Comparison with previous runs

Table 7: Cross-run comparison

| Metric | eng08 (run 3) | eng09 (run 4) | eng10 (run 5) | eng12 (run 6) |
|--------|---------------|---------------|---------------|---------------|
| Spec baseline | v0.20260226.1 | v0.20260226.2 | v0.20260227.3 | v0.20260227.3 |
| Seeded LOC | 0 | 0 | 4,586 | 0 |
| Measure invocations | 3 | 9 | 3 | 3 |
| Measure cost | $3.12 | $6.14 | $2.98 | $2.29 |
| Tasks completed | 1/3 | 0/3 | 4/5 | 2/3 |
| Dominant failure | 15m timeout | validation loops | task oversizing | rate limit pauses |
| 5-hour quota hit | no | yes | no | yes |
| Benchmark valid | no (timeout) | no (0 LOC) | no (seeded) | yes (first clean run) |

## Recommendations

1. Resolve GH-33 before the next generation run. The stitch agent should read prd001-testutils
   as required_reading on every cmd/ task and import the existing package rather than
   recreating it. Until this is resolved, task 21v will continue to fail due to infrastructure
   cost alone.

2. Pre-commit pkg/testutils and pkg/sys to the generation branch in mage generator:start.
   The generation branch now has simplified versions. The correct approach is to commit
   the full versions (matching the feature branch) at start time so stitch agents find
   them immediately. This can be automated as part of the generator:start magefile target.

3. Extend the stitch window or add retry budget for tasks with known infrastructure costs.
   The current 15-minute window is insufficient when rate limit pauses add 10-15 minutes.
   If the cobbler-scaffold timeout is configurable, increase it for complex cmd/ tasks.

4. Start generation runs at times when a fresh API quota window is available. The measure
   phase costs ~250K output tokens. Starting a run with less than 250K output tokens
   remaining in the 5-hour window will cause the same quota exhaustion seen in this run.

5. Task 21v (cmd/ls -i,-s,-n,-F) remains open on generation branch
   generation-2026-02-27-21-48-32. After resolving GH-33, resume with
   mage generator:run 20 on that branch. The branch includes the pre-committed
   pkg/testutils and pkg/sys.

## References

- docs/engineering/eng10-generation-run-5-results.yaml — Run 5 results
- docs/engineering/eng11-stitch-task-sizing.yaml — Task sizing guideline
- https://github.com/petar-djukic/go-unix-utils/issues/29 — GH-29: Clean end-to-end run
- https://github.com/petar-djukic/go-unix-utils/issues/31 — GH-31: PRD atomicity
- https://github.com/petar-djukic/go-unix-utils/issues/33 — GH-33: prd001-testutils API reference
- v0.20260227.3 — Specification baseline tag
- generation-2026-02-27-21-48-32 — Generation branch
