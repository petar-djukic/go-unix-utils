# GH-24 Generation Run 4: max_requirements_per_task Validation and Measure Agent Splitting Failure

We ran the cobbler pipeline against the v0.20260226.2 specification baseline on the
gh-24-generation-run-4 feature branch. This run was the first to use cobbler-scaffold
v0.20260226.3, which introduced the max_requirements_per_task validation feature
(cobbler-scaffold#70). Configuration set max_requirements_per_task=4 with
enforce_measure_validation=true and max_measure_retries=2.

The pipeline completed the measure phase (3 iterations, 9 Claude invocations) but
produced 0 lines of code. The stitch phase started one task (cmd/cat) but was killed
after ~2m47s when we stopped the pipeline manually. The dominant finding is that
max_requirements_per_task validation fires correctly but does not achieve the desired
task splitting. The measure agent re-proposes the same monolithic task on each retry,
reducing requirement count marginally (8 to 7 to 5) without splitting into multiple
smaller tasks. After retries exhaust, the oversized task is accepted with warnings,
producing the same outcome as if validation were disabled.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Baseline tag | v0.20260226.2 |
| Generation branch | generation-2026-02-26-23-16-32 |
| V1 tag | v1.20260226.7 |
| Model | Claude Sonnet 4.6 (measure) |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 3 (iterative, 1 per iteration) |
| Max stitch issues per cycle | 10 |
| Estimated lines | 150-250 |
| CLAUDE_CODE_MAX_OUTPUT_TOKENS | 128,000 |
| cobbler-scaffold version | v0.20260226.3 |
| max_requirements_per_task | 4 |
| enforce_measure_validation | true |
| max_measure_retries | 2 |
| Context mode | Default (standard 9-glob discovery, 27 files) |
| Initial prompt bytes | ~289,160 (measure) |

Configuration changes from eng08:
- cobbler-scaffold upgraded from v0.20260226.2 to v0.20260226.3
- max_requirements_per_task set to 4 (new feature from cobbler-scaffold#70)
- enforce_measure_validation set to true (new feature)
- max_measure_retries set to 2 (new feature)

## Measure phase results

The iterative measure ran 3 iterations. Each iteration exhausted all retries before
accepting its task with validation warnings. No iteration produced a task that passed
validation.

Table 2: Measure iteration 1 (pkg/testutils R1-R5)

| Attempt | Requirements | Validation | Cost | Duration |
|---------|-------------|------------|------|----------|
| 1 | 8 | rejected (max 4) | $0.67 | 2m18s |
| Retry 1 | 7 | rejected (max 4) | $0.66 | 2m2s |
| Retry 2 | 5 | rejected (max 4), retries exhausted, accepted | $0.65 | 2m16s |

Task `dtl` created: "Implement pkg/testutils differential testing harness (prd001 R1-R5)"

Table 3: Measure iteration 2 (cmd/ts R1-R8)

| Attempt | Requirements | Validation | Cost | Duration |
|---------|-------------|------------|------|----------|
| 1 | 7 | rejected (max 4 + 9 AC outside P9 range) | $0.75 | 3m13s |
| Retry 1 | 8 | rejected (max 4) | $0.67 | 2m18s |
| Retry 2 | 8 | rejected (max 4), retries exhausted, accepted | $0.70 | 4m14s |

Task `imv` created: "Implement cmd/ts timestamp utility (prd004-ts R1-R8)"

Table 4: Measure iteration 3 (cmd/ts R9, then cmd/cat R1-R5)

| Attempt | Task proposed | Requirements | Validation | Cost | Duration |
|---------|--------------|-------------|------------|------|----------|
| 1 | cmd/ts tests (R9) | 8 | rejected (max 4 + P7 violation) | $0.69 | 2m41s |
| Retry 1 | cmd/ts tests (R9) | 5 | rejected (max 4 + P7 violation) | $0.66 | 2m15s |
| Retry 2 | cmd/cat (R1-R5) | 5 | rejected (max 4), retries exhausted, accepted | $0.69 | 2m24s |

Task `bp8` created: "Implement cmd/cat concatenate utility (prd006-cat R1-R5)"

Total measure cost: $6.14. Total duration: ~24 minutes. 9 Claude invocations.

## Stitch phase results

Table 5: Stitch task execution

| Task | Status | Cost | Duration | LOC (prod) | LOC (test) |
|------|--------|------|----------|------------|------------|
| bp8 (cmd/cat, R1-R5) | killed | $0 | ~2m47s | 0 | 0 |
| dtl (pkg/testutils, R1-R5) | not attempted | -- | -- | -- | -- |
| imv (cmd/ts, R1-R8) | not attempted | -- | -- | -- | -- |

The stitch picked bp8 (cmd/cat) as its first task. The pipeline was manually stopped
after ~2m47s because the run was being used primarily to evaluate the measure validation
feature. The stitch agent was still processing its first turn when killed.

## Validation failure analysis

The max_requirements_per_task=4 validation mechanism works as designed at the
infrastructure level: it correctly counts requirements, rejects oversized tasks, re-prompts
the measure agent, and accepts with warnings after retries exhaust. However, it does not
achieve the intended outcome of splitting large tasks into smaller ones.

The measure agent's behavior on retry follows a consistent pattern across all 9 invocations:

1. The agent receives the 289KB prompt and proposes a task covering all relevant requirements
   for the next unimplemented package (e.g., prd001 R1-R5, prd004-ts R1-R8).

2. When re-prompted after rejection, the agent makes minor adjustments: it may reduce the
   requirement count (8 to 7 to 5) by merging or dropping requirements from the YAML output,
   but it does not split the task into two independent tasks.

3. The agent treats the rejection feedback as a formatting constraint, not an architectural
   directive. It tries to make the same task fit the limit rather than decomposing the work.

This behavior reveals a mismatch between the validation mechanism and the measure prompt's
task decomposition instructions. The prompt asks for "one task" per iteration, so the agent
always proposes exactly one task. It cannot propose two tasks in response to a "too many
requirements" rejection because the iteration protocol limits it to one.

The validation also caught two additional issues:
- P9 violation: acceptance criteria count outside 5-8 range (iteration 2, attempt 1)
- P7 violation: file cmd/ts/ts_test.go matches package name (iteration 3, attempts 1-2)

The P7 violation is a naming convention check. When the agent retried iteration 3 for the
third time, it switched from proposing cmd/ts tests to cmd/cat, avoiding the P7 issue
entirely. This shows the agent can change its proposal on retry, but not in the desired
direction (splitting vs. switching topics).

## Aggregate results

Table 6: Run totals

| Metric | Value |
|--------|-------|
| Total captured cost | $6.14 |
| Measure cost | $6.14 (9 invocations across 3 iterations) |
| Stitch cost | $0 (killed before completion) |
| Total wall clock | ~27 min |
| Productive time | 0 min (no code produced) |
| Wasted time | ~27 min |
| Total LOC (prod) | 0 |
| Total LOC (test) | 0 |
| Tasks completed | 0 of 3 |
| V1 tag | v1.20260226.7 |

## Comparison with eng04, eng07, and eng08

Table 7: Cross-run comparison

| Metric | eng04 (run 1) | eng07 (run 2) | eng08 (run 3) | eng09 (run 4) |
|--------|---------------|---------------|---------------|---------------|
| Specification baseline | v0.20260225.2 | v0.20260226.0 | v0.20260226.1 | v0.20260226.2 |
| cobbler-scaffold | pre-versioned | pre-versioned | v0.20260226.2 | v0.20260226.3 |
| Measure invocations | 16 | 2 | 3 | 9 |
| Total cost | $33.29 | $3.92 | $3.12+ | $6.14 |
| Total LOC (prod) | 2,842 | 280 | 0 | 0 |
| Total LOC (test) | 1,744 | 0 | 178 | 0 |
| Tasks completed | 7/7 | 1/7 | 1/3 | 0/3 |
| Dominant failure | re-proposal waste | 32k output token limit | 15m timeout | validation retry waste |
| Cost per useful LOC | $7.26 | $14.00 | $17.53 | infinity |

Run 4 is the first run to produce zero useful output. The measure validation feature
tripled measure invocations (from 3 to 9) without changing the final task decomposition.
Every task was eventually accepted with the same oversized structure it would have had
without validation, but at 3x the measure cost.

## Root cause and diagnosis

The max_requirements_per_task feature has a structural limitation: the measure prompt
requests exactly one task per iteration, and the validation rejects tasks that exceed the
requirement limit. But the re-prompt does not tell the agent to propose a smaller subset
of the requirements; it only says the previous output was rejected. The agent interprets
this as "reformat" rather than "split."

For task splitting to work, one of these changes is needed:

1. Allow the measure agent to propose multiple tasks per iteration. When a task exceeds
   max_requirements_per_task, the agent should split it into two tasks within the same
   iteration. This requires changing the iteration protocol from "limit=1" to "limit=N."

2. Include the rejection reason in the re-prompt feedback. Tell the agent explicitly:
   "Your task had 8 requirements but the maximum is 4. Split it into two tasks of 4
   requirements each." This requires the re-prompt to carry the validation error message.

3. Lower max_requirements_per_task to match the PRD structure. The PRDs have 5-8
   requirements per package. Setting max_requirements_per_task=4 guarantees rejection
   for every PRD. Setting it to 8 would pass everything and provide no splitting benefit.
   The feature needs a different approach than a hard cap on requirement count.

## Issues discovered

Table 8: Issues discovered or confirmed

| Issue | Description | Impact | Status |
|-------|-------------|--------|--------|
| Validation does not cause splitting | Measure agent re-formats rather than splits when task is rejected | max_requirements_per_task adds cost without changing outcomes | New finding |
| Iteration protocol limits to 1 task | Measure produces exactly 1 task per iteration, cannot split on retry | Structural limitation of current measure design | New finding |
| Re-prompt lacks rejection context | Agent does not receive specific rejection reason on retry | Agent cannot adjust behavior intelligently | New finding |
| P7 violation on cmd/ts test files | ts_test.go matches package name convention | Agent avoids by switching to different package | Confirmed |
| Issue tracker merge warning | generator:stop merge import branch had conflicts | Issue tracking may lose data | Confirmed from eng08 |
| scaffold:push overwrites config | writeScaffoldConfig() destroys custom configuration.yaml settings | Must manually restore config after every scaffold:push | Confirmed from this session |

## Recommendations

1. Disable max_requirements_per_task validation for the next run. Set it to 0 (unlimited)
   or raise it to 8 to avoid wasting measure invocations on un-achievable splitting. The
   feature needs a redesign before it provides value.

2. Redesign task splitting as a multi-task iteration. Change the measure protocol to allow
   the agent to propose 2-3 tasks per iteration when a package has more than 4 requirements.
   The iteration would produce a list of tasks instead of a single task.

3. Add rejection context to re-prompts. When a measure output fails validation, include
   the specific error messages in the retry prompt so the agent can make informed
   adjustments.

4. Fix scaffold:push config overwrite. The writeScaffoldConfig() function in cobbler-scaffold
   should merge new fields into the existing configuration.yaml rather than replacing it
   with DefaultConfig(). This session required manual restoration of all tuning parameters
   after running scaffold:push.

5. Consider removing the retry mechanism entirely. Since the agent cannot comply with the
   constraint, retries just burn tokens. A "validate and warn" mode (no retries, always
   accept) would produce the same tasks at 1/3 the cost.

## References

- `docs/engineering/eng04-generation-run-results.yaml` — First generation run metrics
- `docs/engineering/eng07-generation-run-2-results.yaml` — Second generation run metrics
- `docs/engineering/eng08-generation-run-3-results.yaml` — Third generation run metrics
- v0.20260226.2 — Specification baseline tag
- v1.20260226.7 — Generation output tag
- https://github.com/petar-djukic/go-unix-utils/issues/24 — GH-24: Recurring generation run
- https://github.com/petar-djukic/cobbler-scaffold/issues/70 — max_requirements_per_task feature
