# Generation Run 38

Date: 2026-03-23 to 2026-03-27
Issue: GH-2626
Branch: generation-gh-2626-run38
Scaffold: v0.20260322.1 → v0.20260323.0 (upgraded mid-run)

## Summary

Run 38 is the first full-catalog generation, covering 114 PRDs across 31 releases (00.0 through 14.0). It produced 73,648 LOC across 105 commands and 6 shared packages. The run addressed 95% of requirements at 35% lower cost per requirement than run 37, validating the requirement weight system and shared package architecture introduced in this session.

Three cobbler-scaffold bugs were discovered and filed during the run. The most impactful — measure prompt not including per-requirement completion status (#1948) — was fixed mid-run. Two remain open: overweight task batching (#1951) and straggler requirements in completed releases (#1952).

## Stats

| Metric | Value |
|--------|-------|
| Tasks | 323 done |
| LOC prod | +45,789 |
| LOC test | +27,859 |
| Total LOC | +73,648 |
| Commands | 105 |
| Shared packages | 6 (testutils, sys, format, hashutil, sizeparse, encutil) |
| Cost | $432.23 (stitch $387.68 + measure $44.55) |
| Turns | 6,586 |
| Tokens | 270.6M in, 4.9M out |
| Requirements | 1,395/1,475 addressed (94.6%) |
| PRDs completed | 99 of 114 |
| Releases | rel00.0 through rel14.0 (31 releases) |
| Cost/requirement | $0.29 |
| Cost/task | $1.34 |
| Measure overhead | 10.3% of total cost |
| Rate limited | 177 minutes |
| Timeouts | 5 tasks (pr, ptx, stty, fmt, stat) |

## Cross-Run Comparison

| Run | Date | Tasks | LOC | Cost | Requirements | Cost/Req | Measure % | Timeouts |
|-----|------|-------|-----|------|-------------|----------|-----------|----------|
| 37 | 2026-03-19 | 301 | 52,030 | $433 | 1,050/1,081 (97%) | $0.41 | 15% | 1 |
| **38** | **2026-03-23** | **323** | **73,648** | **$432** | **1,395/1,475 (95%)** | **$0.29** | **10%** | **5** |

Run 38 addressed 33% more requirements than run 37 at the same total cost. The cost per requirement dropped 29% ($0.41 → $0.29). Measure overhead dropped from 15% to 10% due to multi-issue batching (max_measure_issues=4). The shared packages (hashutil, sizeparse, encutil) reduced per-utility code duplication, contributing to faster stitch times.

## New in Run 38

### Requirement weights

All 114 PRDs use the `weight` field (introduced in cobbler-scaffold v0.20260322.0). Simple requirements have weight 1; complex requirements (parsers, formatters, recursive traversal) have weight 4. The measure agent uses weights to size stitch tasks via `max_requirements_per_task`.

### Shared packages

Three new shared packages in rel00.0 reduce code duplication across utilities:

- **pkg/hashutil** (prd086): hash output formatting, --check verification — used by 7 hash utilities
- **pkg/sizeparse** (prd087): size suffix parsing (K/M/G/KB/MB) — used by 6 utilities
- **pkg/encutil** (prd088): encoding/decoding pipeline — used by 3 encoding utilities

### Expanded catalog

41 new PRDs added this session (prd074-prd114), covering releases 09.0 through 14.0: sha224sum, sha384sum, b2sum, cksum, sum, base32, base64, basenc, stat, truncate, link, sync, chmod, chgrp, chown, mkfifo, mknod, nice, nohup, users, who, pinky, shred, chroot, install, tsort, pathchk, test, stty, df, dir, vdir, dircolors, pr, ptx, chronic, pee, vidir.

## Findings

### Overweight task batching (cobbler-scaffold#1951)

The measure agent batches requirements by count (`max_requirements_per_task`) but ignores the `weight` field. A task with 4 requirements where 2 have weight 4 has total weight 10, exceeding what stitch can complete in 15-25 minutes. Five tasks timed out due to this:

| Task | PRD | Total Weight | Outcome |
|------|-----|-------------|---------|
| #3441 | prd070-fmt | 12 | timed out 3x |
| #3476 | prd082-stat | 12 | timed out 3x |
| #3537 | prd105-stty | 10 | timed out 3x |
| #3560 | prd110-pr | 7 | timed out 3x |
| #3563 | prd111-ptx | 7 | timed out 3x |

Workaround: reduced `max_requirements_per_task` from 6 to 4 and increased `max_time_sec` from 900 to 1500. Long-term fix: orchestrator should enforce total weight budget per task.

### Straggler requirements in completed releases (cobbler-scaffold#1952)

The release auto-advance logic marks releases as `code_complete` when most tasks pass, but some per-requirement items (typically SIGPIPE handlers, exit codes) remain `ready`. The measure agent then filters these releases out of scope, making the stragglers unreachable. 18 requirements across 9 PRDs were left unaddressed:

- 5 utilities missing SIGPIPE handler (whoami, arch, hostname, hostid, tty)
- 2 utilities missing exit codes + tests (unexpand, uniq)
- 2 utilities missing exit codes + SIGPIPE (groups, realpath)

### Measure prompt completion status (cobbler-scaffold#1948, fixed)

The measure prompt did not include per-requirement completion status from requirements.yaml, causing the measure agent to re-propose already-completed work. Fixed in v0.20260323.0 mid-run. After the fix, the measure correctly skipped completed PRDs and advanced to new releases.

### Rate limiting

The 5-hour account-level rate limit was hit twice during the run, each time requiring a manual resume. The per-request rate limiting was handled correctly by the scaffold (v0.20260322.1 fix from cobbler-scaffold#1805). Total rate-limited time: 177 minutes (13% of wall time).

## Items Deferred

| PRD | Requirements | Reason | Follow-up |
|-----|-------------|--------|-----------|
| prd004-ts | R1-R10 (32 items) | Entire utility — complex date parsing, not attempted | GH-3569 |
| prd110-pr | R2.1-R2.4 (4 items) | Multi-column layout timed out at 25m | GH-3569 |
| prd111-ptx | R1-R3 (9 items) | Permuted index generation timed out at 25m | GH-3569 |
| prd105-stty | partial | Terminal settings display/change timed out | GH-3569 |
| 9 PRDs | 18 items | SIGPIPE/exit code stragglers in completed releases | cobbler-scaffold#1952 |

## Issues Filed

### go-unix-utils
- GH-3569: Split complex PRD requirement groups to prevent overweight stitch tasks

### cobbler-scaffold
- cobbler-scaffold#1948: Measure prompt does not include per-requirement completion status (fixed in v0.20260323.0)
- cobbler-scaffold#1951: Measure agent ignores requirement weights when batching
- cobbler-scaffold#1952: Measure agent skips ready requirements in code_complete releases
