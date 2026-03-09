# Generation Run 27

Date: 2026-03-09
Issue: GH-634
Branch: generation-gh-634-run27
Scaffold: cobbler-scaffold v0.20260309.3

## Summary

Run 27 was the first generation with requirement-level state tracking (cobbler-scaffold
#1378 and audit fixes #1385-#1389). The run eliminated duplicate task proposals through
batches 1-2 (12 cycles, 0 duplicates). Batch 3 produced 1 duplicate (tasks 660/670 both
targeting prd002-sys R2.5-R2.6). Batch 4 produced 0 LOC — measure kept cycling on rel00.0
PRDs because the magefiles UC was never proposed, blocking release advancement.

The run was stopped after 24 cycles. The generation produced 957 prod + 1,359 test LOC
across 22 files at $20.56 total cost.

## Results

| Package | Type | Prod LOC | Test LOC | Files |
|---------|------|----------|----------|-------|
| pkg/testutils | library | 309 | 764 | 5 + 2 |
| pkg/sys | library | 306 | 191 | 6 + 1 |
| pkg/format | library | 342 | 404 | 4 + 4 |
| **Total** | | **957** | **1,359** | **22** |

0 releases marked implemented. 50 of 876 R-items marked complete (5.7%).

## What Worked

- **Requirement-level state tracking (GH-1378)**: Zero duplicates through the first 12
  cycles (batches 1-2). Measure correctly skipped completed R-items and proposed new ones
  each cycle. This is a dramatic improvement over run 26 (9/15 tasks were duplicates).

- **R-item scanning**: 876 R-items extracted from 58 PRDs at generator:start. All three
  rel00.0 library PRDs (testutils, sys, format) were fully addressed.

- **Cost efficiency in early batches**: Batches 1-2 cost $11.47 for 11 unique tasks and
  2,316 LOC ($4.95/KLOC). No wasted spend on duplicates.

- **Requirements.yaml committed after stitch (#1385)**: Requirement state persisted
  correctly across resume boundaries. No state loss observed.

## What Did Not Work

### Magefiles never proposed (persistent from run 26)

rel00.0-uc001-magefiles requires prd011-magefiles, which has 19 R-items all at status
"ready". Measure never proposed tasks for prd011 across 20 invocations. The magefiles/
directory is not in go_source_dirs, so measure does not recognize it as a generation target.

This blocks all progress: rel00.0 cannot complete without uc001-magefiles, and measure
will not advance past rel00.0 to propose cmd/ tasks (rel01.x and beyond).

### One duplicate slipped through programmatic validation

Tasks 660 and 670 both targeted prd002-sys R2.5-R2.6 with identical titles. The R-items
were marked complete after task 660, so the programmatic validation (#1386) should have
rejected the duplicate proposal in task 670. This suggests the validation check is not
working correctly or the measure output format did not match the expected pattern.

### Diminishing returns in later batches

Batches 3-4 (12 cycles) produced only 0 additional LOC. Tasks 668, 670, 672, 674
generated 0 prod and 0 test lines. Measure was re-proposing variations of already-complete
rel00.0 work because the release could not advance past the magefiles blocker.

### UC completion gate never fired

Despite 50 R-items being marked complete across all three library PRDs, no UCs were
marked implemented. The UC completion check requires all touchpoint PRD R-items to be
complete, but rel00.0 UCs reference prd011-magefiles which has 0 complete R-items.

## Metrics

| Metric | Value |
|--------|-------|
| Cycles | 24 (4 batches of 6) |
| Measure invocations | 20 |
| Stitch tasks | 20 (19 unique, 1 duplicate) |
| Total cost | $20.56 |
| Stitch cost | $13.41 |
| Measure cost | $7.16 |
| Total turns | 279 |
| Tokens in | 9.5M |
| Tokens out | 180K |
| Production LOC | 957 |
| Test LOC | 1,359 |
| Files created | 22 |
| R-items complete | 50 / 876 |
| Releases completed | 0 |

## Comparison

| | Run 26 | Run 27 |
|---|--------|--------|
| Scaffold | v0.20260309.0 | v0.20260309.3 |
| Stitch tasks | 15 (6 unique) | 20 (19 unique) |
| Duplicate rate | 60% (9/15) | 5% (1/20) |
| Total cost | $20.16 | $20.56 |
| Useful cost | ~$10 | ~$19 |
| LOC | 3,437 | 2,316 |
| Releases completed | 0 | 0 |

Run 27 eliminated nearly all duplicates (60% → 5%) but produced less total LOC because
it was blocked on the magefiles problem. The cost was similar but far less was wasted.

## Issues to File

| Issue | Title | Repo |
|-------|-------|------|
| TBD | Measure never proposes prd011-magefiles tasks | cobbler-scaffold |
| TBD | Programmatic dedup validation missed duplicate prd002-sys R2.5-R2.6 | cobbler-scaffold |

## Next Steps

1. Fix the magefiles blind spot: either add magefiles/ to go_source_dirs, teach measure
   to recognize non-cmd/pkg targets, or exclude prd011-magefiles from the generation
   (implement manually).
2. Investigate why the programmatic validation (#1386) did not reject the duplicate
   prd002-sys R2.5-R2.6 proposal.
3. Consider decoupling rel00.0-uc001-magefiles from the release gate so that library
   UCs (format, sys, testutils) can complete independently.
4. Run 28 after the magefiles issue is resolved.
