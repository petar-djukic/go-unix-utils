# Generation Run 26

Date: 2026-03-09
Issue: GH-552
Branch: generation-gh-552-run26
Scaffold: cobbler-scaffold v0.20260309.0

## Summary

Run 26 was the first generation with the v0.20260309.0 scaffold fixes (roadmap reset,
separate measure/stitch issues, UC validation, history cleanup, branch cleanup). The run
produced 3,437 LOC across 6 packages in 18 cycles at $20.16 total cost. Approximately
half the cost was wasted on duplicate task proposals.

## Results

| Package | Type | Prod LOC | Test LOC | Release |
|---------|------|----------|----------|---------|
| pkg/testutils | library | 282 | 325 | rel00.0 |
| pkg/sys | library | 211 | 227 | rel00.0 |
| pkg/format | library | 291 | 312 | rel00.0 |
| cmd/cat | command | 303 | 331 | rel01.1 |
| cmd/sponge | command | 185 | 266 | rel01.2 |
| cmd/du | command | 390 | 310 | rel01.3 |
| **Total** | | **1,662** | **1,771** | |

29 files created. No release marked as implemented (UC validation gate blocked all).

## What Worked

- **Roadmap reset at generator:start (GH-1368)**: All releases started at spec_complete.
  Run 26 correctly began with rel00.0 infrastructure (testutils, sys, format) before
  moving to commands. Previous runs (23-25) skipped rel00.0 entirely.

- **Separate measure/stitch issues (GH-1367)**: Each measure invocation created its own
  GitHub issue, distinct from the stitch task issue. Stats showed both clearly.

- **du completed successfully**: The sub-requirement counting fix (GH-1349 from v0.20260308.6)
  worked. du completed in 9 minutes / 82 turns / $3.05, compared to timing out at 15 minutes
  in run 22.

- **History cleanup (GH-1356)**: generator:start and generator:stop both cleaned history.
  stats:generator reported only current-run data.

- **Branch cleanup (GH-1359)**: generator:stop deleted the generation branch. No stale
  branches after stop.

## What Did Not Work

### Duplicate task proposals

15 stitch tasks ran but only 6 unique packages were built. 9 tasks were duplicates:

| Package | Times built | Stitch issues |
|---------|-------------|---------------|
| testutils | 3 | #602, #616, #628 |
| sys | 3 | #604, #618, #620 |
| format | 2 | #606, #622 |
| cat | 3 | #609, #610, #624 |
| sponge | 2 | #612, #626 |
| du | 2 | #614, #630 |

Root cause: measure cannot see closed issues in its prompt context. After stitch closes a
task, the next measure invocation has no visibility into completed work. Claude proposes the
same PRD with a slightly different title, bypassing title-based dedup.

File-based dedup (GH-1373) did not prevent cross-batch duplicates because the earlier issues
were already closed.

Estimated waste: ~$10 (half of total cost).

### Releases never marked implemented

UC validation (GH-1361) requires test files to exist and pass before marking a UC as
implemented. Several stitch tasks produced production code without tests (e.g. testutils
#602: +272 prod, 0 test). Without tests, the UC stays at spec_complete, the release is
never marked implemented, and measure keeps proposing the same release.

0 of 80 use cases marked implemented despite 6 packages being fully built.

### Magefiles never proposed

rel00.0-uc001-magefiles was never proposed by measure across 16 invocations. The magefiles
PRD (prd011-magefiles) exists but measure consistently chose testutils, sys, and format
instead. This may be because measure picks PRDs that map to cmd/ or pkg/ packages and
does not recognize magefiles/ as a target.

### Rel column empty in stats

Most stitch tasks showed `rel -` instead of their actual release. The release association
logic in stats:generator could not map task titles to releases.

## Metrics

| Metric | Value |
|--------|-------|
| Cycles | 18 (3 batches of 6) |
| Measure invocations | 16 |
| Stitch tasks | 15 (6 unique, 9 duplicates) |
| Total cost | $20.16 |
| Stitch cost | $15.81 |
| Measure cost | $4.35 |
| Total turns | 449 |
| Tokens in | 14.1M |
| Tokens out | 199K |
| Production LOC | 1,662 |
| Test LOC | 1,771 |
| Files created | 29 |
| Releases completed | 0 |

## Comparison with Run 20

| | Run 20 | Run 26 |
|---|--------|--------|
| Commands | 38 | 3 |
| LOC | 18,170 | 3,437 |
| Issues | 52 | 15 (6 unique) |
| Releases | 17 | 0 marked |
| Scaffold | v0.20260307.1 | v0.20260309.0 |

Run 20 used an earlier scaffold without UC validation or requirement tracking. It marked
releases aggressively (even when incomplete) but produced far more code. Run 26 is more
correct but less productive due to the duplicate waste and completion detection gap.

## Issues Filed

| Issue | Title | Status |
|-------|-------|--------|
| scaffold#1356 | generator:stop does not clear history directory | Fixed v0.20260308.7 |
| scaffold#1359 | generator:stop does not delete generation branch | Fixed v0.20260308.8 |
| scaffold#1360 | measure invocations tracked as GitHub issues | Fixed v0.20260308.8 |
| scaffold#1361 | stitch marks entire release as implemented after single task | Fixed v0.20260308.8 |
| scaffold#1365 | stats:generator show in-progress measure | Fixed v0.20260309.0 |
| scaffold#1366 | unify stats data sources via history files | Fixed v0.20260309.0 |
| scaffold#1367 | separate measure and stitch issues | Fixed v0.20260309.0 |
| scaffold#1368 | generator:start reset roadmap statuses | Fixed v0.20260309.0 |
| scaffold#1373 | measure duplicate tasks with different PRD naming | Fixed |
| scaffold#1374 | requirement-level state tracking | Open |
| scaffold#1375 | stats:generator inconsistent issue formatting | Open |
| scaffold#1378 | end-to-end requirement tracking and completion detection | Open |

## Next Steps

1. Implement requirement-level state tracking (scaffold#1378) to eliminate duplicates
   and enable accurate release completion detection.
2. Ensure stitch produces tests alongside production code, or decouple UC completion
   from test existence.
3. Investigate why magefiles PRD is never proposed by measure.
4. Run 27 after scaffold#1378 is resolved.
