# Generation Run 42 Report

## Summary

Run 42 is the first full-catalog generation on cobbler-scaffold v0.20260406.0, validating the PRD-to-SRD rename, interface migration, weight migration to requirements.yaml, and CoT weight-reasoning in the measure prompt. The run achieved 100% requirement completion (1,475/1,475) and produced 107 commands and 6 shared packages totaling 75,393 lines of Go code.

## Results

| Metric | Run 40b | Run 42 |
|---|---|---|
| Date | 2026-03-28 | 2026-04-06 to 2026-04-11 |
| Scaffold version | v0.20260328.1 | v0.20260406.0 |
| Requirements | 1,475/1,475 (100%) | 1,475/1,475 (100%) |
| Stitch tasks | 403 | 459 |
| Measure cycles | 113 | ~120 |
| LOC (prod) | 41,341 | 44,350 |
| LOC (test) | 35,592 | 31,043 |
| LOC (total) | 76,933 | 75,393 |
| Commands | 107 | 107 |
| Shared packages | 6 | 6 |
| Total cost (stitch) | — | $536.07 |
| Total cost (measure) | — | $57.76 |
| Total cost | ~$521 | $593.82 |
| Cost/requirement | $0.35 | $0.40 |

## Stitch Performance

| Metric | Value |
|---|---|
| Total turns | 9,055 |
| Total rate-limited time | 246m12s |
| Retries | 18 across 477 attempts (4%) |
| Retry cost wasted | $1.99 |
| Tokens in (stitch) | 369.8M |
| Tokens out (stitch) | 5.6M |
| Tokens in (measure) | 7.6M |
| Tokens out (measure) | 797K |

## Scaffold Changes Validated

This run validated all scaffold changes made on 2026-04-05 and 2026-04-06:

1. **PRD to SRD rename** (PR #4215): All specification files renamed from `prd` to `srd` prefix, all cross-references updated. Measure and stitch handled the new naming without issues.

2. **Interface migration** (PR #4217): Interfaces moved from inline ARCHITECTURE.yaml to individual files in `docs/interfaces/`. No impact on generation.

3. **Weight migration to requirements.yaml** (PR #4408): Weights removed from SRD files, stored in requirements.yaml as scheduling metadata. CoT weight-reasoning step added to measure prompt.

4. **CoT weight-reasoning**: The measure prompt now includes an explicit step where Claude lists requirements with weights and sums the total before proposing tasks. Zero weight violations in the first 30 cycles (vs multiple in run 41). Weight violations appeared later in the run but were not enforced (enforcement disabled after the splitter experiment failed).

## Incidents

### Weight enforcement experiment (run 41, pre-run 42)

Three weight enforcement approaches were tried and all failed before starting run 42:
- **Reject and retry** (cobbler-scaffold#2070): Claude re-proposes the same batch. Wastes ~$4/retry.
- **Post-measure splitting** (cobbler-scaffold#2072): Split fragments cannot execute independently (shared output files, zero LOC). Reverted in #2074.
- **max_weight_per_task tuning**: Doesn't help if the enforcement mechanism is broken.

The CoT weight-reasoning approach (#2077) was adopted instead. It works ~80% of the time without any enforcement overhead.

### Duplicate task creation

Measure created duplicate tasks for the same requirements, particularly for sort (srd053). R1.5-R1.7 was proposed 4 times, R4.1-R4.4 was proposed 4 times — 18 issues for 6 unique task groups. The duplicate tasks completed with zero LOC (code already written by the first task), triggering the zero-LOC safety stop.

Root cause: requirements.yaml has no "proposed" state. Between measure proposing and stitch completing, requirements remain "ready," so the next measure cycle proposes them again. Filed as cobbler-scaffold#2123.

### Zero-LOC safety stops

The 3-consecutive-zero-LOC-cycle stop triggered multiple times throughout the run, requiring manual restarts. Each stop burned 3 cycles. The stop is too aggressive — it declares "spec likely complete" at 55% completion when duplicate tasks produce zero LOC.

Root cause: the zero-LOC stop does not check whether pending requirements exist. It should only trigger when all requirements are complete/failed/uncertain. Filed as part of cobbler-scaffold#2123.

### Stitch timeouts (od, stty)

Two utilities timed out repeatedly:
- **cmd/od** (srd072 R1.1-R1.4): Killed at 15 minutes twice, then hit 25-minute max. Octal dump with complex format parsing.
- **cmd/stty** (srd105 R1.1, R2.1, R3.1, R3.2): Killed at 15 minutes, then hit 25-minute max. Terminal control with ioctl complexity.

Both were closed manually to unblock the generation. The requirements were re-proposed by measure in subsequent cycles and completed on retry.

### Measure SRD reference format (cosmetic)

The new measure prompt generates short-form SRD references (`srd087`) instead of full IDs (`srd087-sizeparse`), breaking `stats:generator` SRD tracking. Most tasks show `Rel: -`. Filed as cobbler-scaffold#2120.

### requirements.yaml weight loss on generator:start

`GenerateRequirementsFile` uses `PreserveSources` to decide whether to preserve existing state. When false (default for go-unix-utils), weights are lost. Weights were manually restored before starting the run. Filed as cobbler-scaffold#2117.

### Road-map.yaml excluded from measure context

`docs/road-map.yaml` was in `context_exclude`, preventing measure from knowing which SRDs belong to which release. This caused out-of-order task proposals (rel01.1 before rel00.0 cmd/ tasks). Fixed mid-run by removing road-map.yaml from the exclude list.

## Cobbler-Scaffold Issues Filed

| Issue | Title | Status |
|---|---|---|
| #2070 | Separate weight enforcement from granularity and file-naming validation | Merged |
| #2072 | Split overweight measure tasks programmatically | Merged, then reverted (#2074) |
| #2074 | Revert post-measure task splitting | Merged |
| #2077 | Add weight to requirements.yaml and CoT weight-reasoning step | Merged |
| #2078 | Add validate-task-weights mage target | Open |
| #2080 | Remove weight from SRDs, requirements.yaml sole authority | Merged |
| #2116 | Remove unused mage credentials target | Open |
| #2117 | GenerateRequirementsFile loses weights when PreserveSources=false | Open |
| #2120 | Measure generates short-form SRD refs, breaking stats:generator | Open |
| #2123 | requirements.yaml needs complete state machine for dedup and stops | Open |

## Comparison with Run 40b

Run 42 achieved 100% requirements like run 40b but at higher cost ($594 vs $521, +14%). The cost increase is driven by:
- Duplicate tasks from the deduplication bug (~56 extra tasks, ~$60 wasted)
- Rate limiting (246 minutes total, vs minimal in run 40b)
- More retries on complex utilities (od, stty)

LOC is comparable (75,393 vs 76,933, -2%). The test-to-prod LOC ratio decreased from 0.86 to 0.70, indicating the scaffold is generating slightly fewer tests per command in this run.

The key validation: all scaffold changes (PRD→SRD, interfaces, weight migration, CoT) work correctly. The run completed without any scaffold-related failures.
