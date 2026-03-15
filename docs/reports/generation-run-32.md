# Generation Run 32

Date: 2026-03-14
Issue: GH-1297
Tag: generation-gh-1297-run32-finished
Scaffold: v0.20260314.0

## Summary

Test run to validate cobbler-scaffold v0.20260314.0 upgrade and investigate stitch turn efficiency. Completed 13 stitch tasks across prd001-testutils, prd002-sys, and prd003-format. The run served as a controlled experiment for prompt optimization: reducing stitch turns from 24 to 9 per task by excluding docs/ from the stitch context.

## Stats

| Metric | Value |
|--------|-------|
| Tasks | 13 done |
| LOC prod | +672 |
| LOC test | +1,246 |
| Total LOC | +1,918 |
| Cost | $12.23 (stitch $7.80 + measure $4.44) |
| Turns | 235 |
| Requirements | 42/867 (4%) |

## Findings: Stitch Turn Reduction

### Problem

Stitch tasks averaged 24 turns due to two issues:
1. Claude reads PRDs and source files already provided inline in the prompt (3-5 wasted turns per task)
2. Trial-and-error test writing: write test, fail, fix, repeat (3-6 wasted turns per task)

### Experiments

| Approach | Turns | Cost | Result |
|----------|-------|------|--------|
| Baseline (task #1303) | 24 | $0.78 | n/a |
| Prompt wording fix (task #1313) | 22 | $0.73 | No improvement |
| Exclude docs/ from context (task #1315+) | 9-14 | $0.30-0.44 | 40% turn reduction |

### Root Cause

The `repository_files` listing in the stitch prompt showed 187 docs/ file paths. Claude's verification loop triggered Read/Glob/Grep calls for these paths regardless of prompt instructions. Excluding docs/ from context removed the paths from the listing, eliminating the trigger.

### Issues Filed

- cobbler-scaffold#1463: Prompt wording alone does not prevent redundant reads
- cobbler-scaffold#1464: Stop injecting PRDs into project_context, let Claude read them via required_reading

## Configuration Changes

- `context_exclude`: replaced individual docs/ entries with `docs/` (covers all subdirs)
- `docs/prompts/stitch.yaml`: added tool-specific constraints, precondition reasoning, first-attempt coding instructions
