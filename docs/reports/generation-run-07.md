# GH-36 Generation Run 7: Per-Release Generation with Pipeline Bug Discovery

We ran the cobbler pipeline under a new per-release orchestration model (GH-36) on generation
branch generation-2026-02-28-11-56-07. The goal was to generate code one release at a time,
starting with rel00.0 (foundational infrastructure), then proceeding through rel01.0, rel01.1,
and rel02.0 sequentially. This replaced the previous all-releases-at-once approach from GH-29.

The rel00.0 generation succeeded, producing 892 production LOC and 1054 test LOC across three
packages (pkg/format, pkg/sys, pkg/testutils). The measure agent correctly proposed
infrastructure tasks and the stitch agent completed them within budget. All generated code
compiles and tests pass.

The rel01.0 generation failed due to pipeline bugs in the cobbler-scaffold. The measure agent
ignored the releases configuration filter and proposed tasks outside the target release. A
issue tracker history leak caused the measure agent to see stale completed tasks, leading it to skip
rel01.0 entirely. We filed six cobbler-scaffold bugs and two go-unix-utils roadmap
restructuring issues. Generation was stopped after rel00.0 to avoid wasting further cycles
on broken pipeline components.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Spec baseline tag | v0.20260228.0 |
| Generation branch | generation-2026-02-28-11-56-07 |
| Model | Claude Sonnet 4.6 |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 1 |
| Max stitch issues per cycle | 1 |
| CLAUDE_CODE_MAX_OUTPUT_TOKENS | 128,000 |
| max_requirements_per_task | 4 |
| enforce_measure_validation | false |
| max_measure_retries | 0 |
| releases filter | ["00.0"] (then ["01.0"] for second attempt) |
| context_exclude | SPECIFICATIONS, road-map |
| Seeded LOC | 0 (clean start) |

This run introduced per-release scoping via the configuration.yaml releases field. Each
release was configured individually before running the generator. The releases filter was
expected to constrain which tasks the measure agent could propose.

## rel00.0 generation results

The rel00.0 generation completed in one measure+stitch cycle (with one anomaly). The
measure agent proposed a single task covering pkg/format, pkg/sys, and pkg/testutils as
a combined infrastructure task mapped to rel00.0 use cases uc002, uc003, and uc004.

Table 2: rel00.0 stitch outcome

| Phase | Task ID | Description | Duration | Cost | Outcome |
|-------|---------|-------------|----------|------|---------|
| Stitch | 5w2 | rel00.0 sys, format, testutils | 1m46s | $0.68 | success |

The stitch agent produced 16 Go files across three packages in under 2 minutes. All files
compile and all tests pass. This is the fastest successful stitch we have observed,
suggesting that infrastructure packages with well-specified PRDs and no external
dependencies are well within the stitch agent's budget.

Table 3: Generated packages

| Package | Files | Prod LOC | Test LOC | Description |
|---------|-------|----------|----------|-------------|
| pkg/testutils | 1 | 278 | 291 | DiffTest harness, normalizers |
| pkg/format | 3 | 375 | 435 | Color, columns, size formatting |
| pkg/sys | 5 | 239 | 328 | Terminal, signal, stat (darwin/linux) |
| Total | 9 | 892 | 1054 | |

The magefiles use case (rel00.0-uc001) was not generated because the measure agent
proposed a rel01.0 task (ts, task dx0) instead. This is tracked as cobbler-scaffold #117.

## Measure anomaly: wrong-release task proposal

During the rel00.0 cycle, after the 5w2 stitch completed, the measure agent proposed task
dx0: "rel01.0-uc001 ts: Implement cmd/ts timestamp prepend utility." This task belongs to
rel01.0, not rel00.0. The releases filter was set to ["00.0"] but the measure agent
ignored it.

Task dx0 then failed twice in stitch (both hit the 15-minute timeout with 0 tokens used),
suggesting the API rate limit was exhausted. We closed dx0 as out-of-scope and stopped
the rel00.0 generation.

Table 4: dx0 failure details

| Attempt | Duration | Tokens | Cost | Error |
|---------|----------|--------|------|-------|
| 1 | 15m0s | 0 | $0.00 | claude max time exceeded |
| 2 | 15m0s | 0 | $0.00 | claude max time exceeded |

Filed as cobbler-scaffold #117: measure agent ignores releases config filter.

## rel01.0 generation attempt

After merging the rel00.0 generation to main, we configured releases to ["01.0"] and
started a new generation branch (generation-2026-02-28-13-42-29, then
generation-2026-02-28-13-52-17).

The first attempt failed immediately: the issue tracker init imported 14 stale entries from
a git merge artifact. The measure agent saw dx0 (ts) as COMPLETED and
skipped rel01.0 entirely, proposing a rel01.1 wc task instead. The generator exited after
one cycle saying "no open issues remain."

Filed as cobbler-scaffold #118: issue tracker re-imports stale history on init.

The second attempt required destroying and reinitializing the issue tracker database from
scratch because the SQLite database had become corrupted after multiple resets.
Filed as cobbler-scaffold #120: SQLite DB corruption after multiple issue tracker resets.

After a clean reinit, the measure phase started but we stopped the generator before stitch
could execute, deciding to address the pipeline bugs before spending more cycles.

## Pipeline bugs discovered

This run exposed six pipeline bugs in cobbler-scaffold, all filed as issues:

Table 5: Cobbler-scaffold bugs

| Issue | Title | Severity | Impact |
|-------|-------|----------|--------|
| #117 | Measure agent ignores releases config filter | high | Wrong-release tasks waste stitch cycles |
| #118 | Issue tracker re-imports stale history on init | high | Measure sees completed tasks, skips work |
| #119 | Generator retries failed tasks without changes | medium | Identical retries waste API budget |
| #120 | SQLite DB corruption after multiple issue tracker resets | medium | Requires re-initialization to recover |
| #121 | generator:run should accept cycle count argument | low | Must edit config.yaml instead of CLI arg |
| #122 | Subrequirements in PRDs may prevent measure from splitting | medium | Tasks sized by top-level count, not actual scope |

Bugs #117 and #118 are blockers for per-release generation. Until they are fixed, the
measure agent cannot be reliably constrained to a single release.

## Roadmap restructuring issues

Based on findings from this run and the previous run (eng12), we filed two roadmap
restructuring issues in go-unix-utils:

Table 6: Roadmap issues

| Issue | Title | Rationale |
|-------|-------|-----------|
| #59 | Move ts from rel01.0 to rel99.0 | ts has failed in 7+ stitch attempts across two runs; subrequirements make it too large for current pipeline |
| #60 | Split wc, cat, sponge into separate dot releases | Per-release generation model works better with one utility per release |

## Cost analysis

Table 7: Cost breakdown

| Phase | Cost | Duration | Notes |
|-------|------|----------|-------|
| rel00.0 measure | $0.87 | 7m32s | Proposed 5w2 + dx0 |
| rel00.0 stitch (5w2) | $0.68 | 1m46s | Success: 892 prod + 1054 test LOC |
| rel00.0 stitch (dx0, 2x) | $0.00 | 30m0s | Both timed out with 0 tokens (rate limit) |
| rel01.0 measure (attempt 2) | ~$0.87 | ~7m | Started but killed before completion |
| Total | ~$2.42 | ~46m | Productive time: ~9m |

The effective cost per production LOC was $0.0017 ($1.55 for 892 LOC). This is the lowest
cost per LOC we have achieved, primarily because the infrastructure packages are
well-specified and fit within a single stitch window.

## Comparison with previous runs

Table 8: Cross-run comparison

| Metric | eng10 (run 5) | eng12 (run 6) | eng14 (run 7) |
|--------|---------------|---------------|---------------|
| Spec baseline | v0.20260227.3 | v0.20260227.3 | v0.20260228.0 |
| Seeded LOC | 4,586 | 0 | 0 |
| Target releases | all | all | per-release |
| Measure invocations | 3 | 3 | 2 |
| Measure cost | $2.98 | $2.29 | $1.74 |
| Tasks completed | 4/5 | 2/3 | 1/2 |
| Prod LOC generated | 3,530 | 640 | 892 |
| Test LOC generated | 1,056 | 0 | 1,054 |
| Dominant failure | task oversizing | rate limit pauses | pipeline bugs |

Run 7 is the first to produce both production and test LOC in a single stitch pass for
infrastructure packages. The per-release model reduced measure cost because fewer tasks
were proposed per cycle. However, the releases filter bug (#117) undermined the intended
scoping, and the issue tracker history leak (#118) prevented the second release from starting.

## Recommendations

1. Fix cobbler-scaffold #117 and #118 before the next generation run. These are blockers
   for per-release generation. Without them, the measure agent cannot be reliably scoped
   to a single release.

2. Execute roadmap restructuring (#59, #60) to move ts to rel99.0 and split wc, cat,
   sponge into separate dot releases. This aligns the roadmap with the per-release model
   and avoids the ts failure pattern observed in runs 6 and 7.

3. Address #119 (retry without changes) and #122 (subrequirement counting) to improve
   stitch success rates on complex tasks.

4. The rel00.0 generated code (tagged v1.20260228.2) is production-ready for the
   infrastructure packages. The next run should start from rel01.0 (or rel01.1 after
   roadmap restructuring) and seed with the rel00.0 code.

5. Consider increasing the stitch window beyond 15 minutes for cmd/ tasks that depend on
   shared infrastructure. The 1m46s stitch time for 5w2 shows that well-scoped tasks
   complete far under budget, but tasks that need to build infrastructure first (like dx0)
   cannot fit in the window.

## References

- docs/engineering/eng12-generation-run-6-results.yaml — Previous run results
- https://github.com/petar-djukic/go-unix-utils/issues/36 — GH-36: Per-release generation
- https://github.com/petar-djukic/go-unix-utils/issues/53 — GH-53: rel00.0 generation
- https://github.com/petar-djukic/cobbler-scaffold/issues/117 — Measure ignores releases filter
- https://github.com/petar-djukic/cobbler-scaffold/issues/118 — Issue tracker stale history on init
- https://github.com/petar-djukic/cobbler-scaffold/issues/119 — Retry without changes
- https://github.com/petar-djukic/cobbler-scaffold/issues/120 — SQLite DB corruption
- https://github.com/petar-djukic/cobbler-scaffold/issues/121 — generator:run cycle count arg
- https://github.com/petar-djukic/cobbler-scaffold/issues/122 — Subrequirement splitting
- v1.20260228.2 — rel00.0 generated code tag
- generation-2026-02-28-11-56-07 — Primary generation branch
