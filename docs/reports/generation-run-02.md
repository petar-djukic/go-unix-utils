# GH-16 Generation Run 2: Results and Output Token Limit Failure

We ran the cobbler pipeline against the v0.20260226.0 specification baseline on the
gh-16-full-generation-run feature branch. The run used default context configuration
(standard 9-glob discovery resolving 27 files) with Claude Opus 4.6 at temperature 0.

The pipeline produced 1 of 7 target packages (pkg/testutils, 280 production LOC) before
encountering a blocking infrastructure issue: the Claude Code container enforces a 32,000
output token maximum (CLAUDE_CODE_MAX_OUTPUT_TOKENS), and the stitch agent exceeded this
limit when attempting to implement cmd/ts. The agent produced the entire source file in a
single Write tool call, generating >32k tokens of output on turn 1, which triggered an API
error. After the error the agent entered a rate-limit backoff, sat idle until the 15-minute
timeout, and was killed. The pipeline retried the same task three times with identical results
before we stopped it manually.

The successful pkg/testutils implementation and the cmd/ts failure together reveal a threshold:
tasks producing ~280 LOC in 2 files fit within the 32k limit, while tasks producing ~300+ LOC
with tests and differential test harness integration do not. The previous eng04 generation run
succeeded because those stitch agents split their output across multiple turns and Write calls.
The current run's agent attempted a single-turn write, which is a non-deterministic behavior
difference under temperature 0.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Baseline tag | v0.20260226.0 |
| Generation branch | generation-2026-02-26-20-39-40 |
| V1 tag | v1.20260226.5 |
| Model | Claude Opus 4.6 |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 1 |
| Max stitch issues per cycle | 10 |
| Estimated lines | 250-350 |
| Context mode | Default (standard 9-glob discovery, 27 files) |
| Initial prompt bytes | ~286,595 (measure), ~283,743 (stitch) |

Pre-flight configuration fixes applied before the run:
- go_source_dirs restored to [pkg, cmd] (was [] due to prior generation cleanup)
- max_time_sec restored to 900 (was 300, which caused eng04 timeouts)

## Per-cycle results

Table 2: Cycle-by-cycle metrics

| Cycle | Phase | Task | Status | Cost | Duration | LOC (prod) | LOC (test) |
|-------|-------|------|--------|------|----------|------------|------------|
| 1 | stitch | (no tasks) | skipped | $0 | 1s | 0 | 0 |
| 1 | measure | propose pkg/testutils | success | $0.64 | 2m | 0 | 0 |
| 2 | stitch | pkg/testutils (prd001 R1-R5) | success | $2.61 | 10m | +280 | 0 |
| 2 | measure | propose cmd/ts | success | $0.67 | 2m | 0 | 0 |
| 3 | stitch | cmd/ts (prd004-ts R1-R8) | failed | $0 | 15m | 0 | 0 |
| 3 | stitch | cmd/ts (retry 1) | failed | $0 | 15m | 0 | 0 |
| 3 | stitch | cmd/ts (retry 2) | killed | $0 | ~12m | 0 | 0 |

Note: failed stitch cycles report $0 cost because the orchestrator could not parse token
usage from the killed Claude process. The actual token consumption is non-zero (the agent
produced >32k output tokens before the API error) but was not captured in stats.

## Aggregate results

Table 3: Run totals

| Metric | Value |
|--------|-------|
| Total cost | $3.92 |
| Measure cost | $1.31 (2 cycles) |
| Stitch cost | $2.61 (1 success, 3 failures) |
| Total wall clock | ~56 min |
| Productive time | ~14 min (measure + successful stitch) |
| Wasted time | ~42 min (3 failed stitch attempts) |
| Total LOC (prod) | 280 |
| Total LOC (test) | 0 |
| Packages produced | 1 of 7 (pkg/testutils) |
| V1 tag | v1.20260226.5 |

## Failure analysis

The cmd/ts stitch failure has a clear chain of causation:

1. The measure agent proposed "cmd/ts: Implement timestamp stdin lines (prd004-ts R1-R8)"
   as a single task covering all 8 requirements. This is a reasonable task size per the
   250-350 estimated lines guideline.

2. The stitch agent received the task and began implementation. On turn 1, it attempted
   to write the entire cmd/ts implementation in a single response that exceeded Claude
   Code's CLAUDE_CODE_MAX_OUTPUT_TOKENS limit of 32,000 tokens.

3. The API returned an error: "Claude's response exceeded the 32000 output token maximum."
   The agent could not recover from this mid-response truncation.

4. After the API error, the agent entered a rate-limit backoff period. With no further
   productive work possible, it sat idle until the 15-minute timeout killed it.

5. The orchestrator reset the task to "open" and retried. The same agent behavior
   (single-turn write exceeding 32k) repeated on each retry.

The previous eng04 generation run produced cmd/ts successfully because the stitch agent
in that run split its output across multiple turns: first writing a plan, then creating
files one at a time, then running go build/vet. The difference is non-deterministic
agent behavior under temperature 0 — the same prompt can produce different tool-use
strategies across invocations.

## Comparison with eng04

Table 4: eng04 vs eng07 comparison

| Metric | eng04 (run 1) | eng07 (run 2) |
|--------|---------------|---------------|
| Specification baseline | v0.20260225.2 | v0.20260226.0 |
| Total cycles | 16 | 3 (+3 retries) |
| Total cost | $33.29 | $3.92 |
| Total LOC (prod) | 2,842 | 280 |
| Total LOC (test) | 1,744 | 0 |
| Packages produced | 7 | 1 |
| Re-proposal waste | 72% of cycles | 0% (run stopped early) |
| Stitch failures | 0 | 3 |
| Cost per LOC | $7.26 | $14.00 |

The eng04 run completed all 7 target packages despite significant measure re-proposal
waste (cobbler-scaffold#52). The eng07 run produced higher-quality measure output (no
re-proposals in the 2 measure cycles observed) but was blocked by the output token limit.

## Bugs and issues

Table 5: Issues discovered or confirmed

| Issue | Description | Impact |
|-------|-------------|--------|
| 32k output token limit | Claude Code container enforces CLAUDE_CODE_MAX_OUTPUT_TOKENS=32000 | Blocks tasks requiring >32k output tokens in a single response |
| Infinite retry loop | Stitch retries same failing task indefinitely with no retry counter or backoff | Wastes time and API credits on unrecoverable failures |
| parseRequiredReading YAML error | "cannot unmarshal !!map into string" on every stitch prompt | Stitch agent gets no required_reading hints, keeping all source files |
| Stats not captured on failure | Failed stitch reports 0 tokens/cost even though tokens were consumed | Undercounts actual cost |

The 32k limit is the dominant blocker. The other issues are quality-of-life improvements
for the cobbler orchestrator.

## Recommendations

1. Set CLAUDE_CODE_MAX_OUTPUT_TOKENS to 128000 in the podman container configuration.
   This is the maximum supported by Claude Opus 4.6 and removes the artificial constraint.

2. Add a retry counter to the stitch orchestrator. After N consecutive failures on the
   same task (suggested N=2), mark the task as permanently failed and move on to the next
   measure cycle. This prevents the infinite retry loop.

3. Fix the parseRequiredReading YAML unmarshalling. The measure agent generates
   required_reading as a map (key-value pairs with notes) but the stitch parser expects
   a list of strings. Either fix the parser or update the measure prompt to emit the
   expected format.

4. Capture token usage from failed Claude invocations. The stream-json output format
   includes token counts even when the response is truncated.

## References

- `docs/engineering/eng04-generation-run-results.yaml` — First generation run metrics
- `docs/engineering/eng06-context-benchmark-results.yaml` — Context benchmark results
- v0.20260226.0 — Specification baseline tag
- v1.20260226.5 — Generation output tag
- https://github.com/petar-djukic/go-unix-utils/issues/16 — GH-16: Recurring generation run
