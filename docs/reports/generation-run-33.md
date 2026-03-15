# Generation Run 33

Date: 2026-03-15
Issue: GH-1329
Tag: generation-gh-1329-run33-finished
Scaffold: v0.20260314.1

## Summary

Validation run for cobbler-scaffold v0.20260314.1 (PRD exclusion #1464, prompt strengthening #1463). Also tested multi-issue measure (max_measure_issues=3). Completed 24 stitch tasks across 5 PRDs covering rel00.0, rel01.1, and rel02.0.

## Stats

| Metric | Value |
|--------|-------|
| Tasks | 24 done |
| LOC prod | +1,677 |
| LOC test | +2,288 |
| Total LOC | +3,965 |
| Cost | $33.13 (stitch $19.67 + measure $13.46) |
| Turns | 403 |
| Requirements | 83/867 (9%) |
| Measure invocations | 42 |

## Findings

### Scaffold v0.20260314.1 PRD Exclusion

The scaffold's native PRD exclusion (#1464) works correctly. PRDs are no longer injected into project_context; Claude reads them via required_reading. Early tasks averaged 7-11 turns (down from 24 in run 32 baseline).

### Multi-Issue Measure (max_measure_issues=3)

Tested `max_measure_issues=3` with `max_stitch_issues_per_cycle=3`. Findings:

- No efficiency gain: measure cost per stitch task is identical ($0.32) since each iteration is a separate Claude call with limit=1
- Placeholder issue spam: 3 placeholder issues created per cycle instead of 1, filed cobbler-scaffold#1467
- Measure cost was 40% of total ($13.46 of $33.13) due to 42 invocations
- Reverted to max_measure_issues=1 for future runs

### Turn Scaling with Context

Stitch turns scale with accumulated source code:
- Early pkg/ tasks (4 source files): 7-11 turns
- Later pkg/ tasks (18 source files): 11-17 turns
- cmd/ tasks (18+ source files): 19-28 turns

This is the known prompt scaling issue (cobbler-scaffold#1115).

## Issues Filed

- cobbler-scaffold#1467: Measure creates one placeholder issue per iteration, flooding the issue tracker

## Configuration Changes

- Reverted max_measure_issues from 3 to 1
- Reverted stitch prompt and context_exclude to scaffold defaults (v0.20260314.1 handles both natively)
