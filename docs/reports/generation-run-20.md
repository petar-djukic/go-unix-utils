# GH-472 Generation Run 20: Full Regeneration with rel06.0 and rel06.1

We ran generation run 20 to regenerate the entire codebase from specifications, adding
rel06.0 (mkdir, rmdir, mktemp, ln, unlink) and rel06.1 (env, printenv, id, whoami, groups)
for the first time. All 17 releases (rel00.0 through rel06.1) were reset from `implemented`
to `spec_complete` and regenerated from scratch using cobbler-scaffold CLI mode. The run
produced 38 utilities across 90 files, totaling 18,170 lines of code (11,078 production +
7,092 test).

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Baseline tag | generation-gh-472-run20-start |
| Generation branch | generation-gh-472-run20 |
| Finished tag | generation-gh-472-run20-finished |
| Merged tag | generation-gh-472-run20-merged |
| Model | Claude Opus 4.6 (CLI mode) |
| Mode | cli |
| Max time per agent | 900s (15 min) |
| Estimated lines | 300-500 |
| Context mode | headers (measure_source_mode=headers) |
| Releases targeted | 00.0 through 06.1 (17 releases, 38 use cases) |

## Per-release results

Table 2: Release-by-release summary

| Release | Utilities | Status | Notes |
|---------|-----------|--------|-------|
| rel00.0 | pkg/sys, pkg/testutils, pkg/format | automated | |
| rel01.1 | cat | automated | |
| rel01.2 | sponge | automated | |
| rel01.3 | du | automated | |
| rel02.0 | true, false, yes, basename, dirname | automated | |
| rel02.1 | tee | automated | |
| rel02.2 | head, seq | automated | |
| rel03.0 | wc | automated | |
| rel04.0 | ls (core + extended) | automated | |
| rel05.0 | echo, tac, nl, fold | automated | |
| rel05.1 | expand, unexpand | automated | |
| rel05.2 | cut, paste | automated | |
| rel05.3 | uniq, comm | automated | |
| rel05.4 | md5sum, sha1sum, sha256sum, sha512sum | automated | |
| rel05.5 | ts | automated | |
| rel06.0 | mkdir, rmdir, mktemp, ln, unlink | automated | |
| rel06.1 | env, printenv, id, whoami, groups | manual | Stitch prompt exceeded 790KB context limit |

## Final metrics

Table 3: Code and documentation totals

| Metric | Value |
|--------|-------|
| Production LOC | 11,078 |
| Test LOC | 7,092 |
| Total Go LOC | 18,170 |
| Commands (cmd/) | 38 |
| PRD words | 32,487 |
| Test suite words | 15,219 |
| Use case words | 18,944 |
| Generation issues | 52 (all closed) |

## Issues encountered

1. Stitch prompt exceeded context limit (790KB, 80 source files) when attempting rel06.1.
   All 5 utilities (env, printenv, id, whoami, groups) were implemented manually. Filed
   cobbler-scaffold#1115 for stitch source filtering.

2. Stray binaries committed to repo root during manual implementation. Cleaned up in a
   follow-up commit.

3. Duplicate measure pattern continued from previous runs: after each stitch, measure
   proposed a duplicate issue for the just-completed release. Resolved by closing
   duplicates, marking releases implemented, advancing config, and resuming.

## Diff from run 19

Table 4: Delta from run 19

| Metric | Run 19 | Run 20 | Delta |
|--------|--------|--------|-------|
| Production LOC | 9,686 | 11,078 | +1,392 |
| Test LOC | 6,077 | 7,092 | +1,015 |
| Total Go LOC | 15,768 | 18,170 | +2,402 |
| Commands | 27 | 38 | +11 |

New commands in run 20: mkdir, rmdir, mktemp, ln, unlink, env, printenv, id, whoami, groups,
plus the 27 commands regenerated from run 19.
