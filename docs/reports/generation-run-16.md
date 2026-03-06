# Generation Run 16: 1,089 LOC, $4.60 — CLI mode trial on rel02.0 trivial utilities

## Configuration

| Field | Value |
|---|---|
| Generation branch | `generation-gh-313-generate-code` |
| Feature branch | `gh-313-generate-code-from-specs` |
| Base branch | `main` |
| Cobbler version | v0.20260306.3 |
| Model | claude-sonnet-4-6 |
| Mode | cli (first CLI mode run; previous runs used podman) |
| Releases targeted | `["02.0"]` |
| Cycles configured | 6 |
| Cycles executed | 4 |
| max_stitch_issues_per_cycle | 1 |
| max_measure_issues | 1 |

## Summary

We ran generation run 16 as an experimental CLI mode trial targeting rel02.0 (true, false, yes,
basename, dirname). The run produced 3 tasks and 1,089 LOC in 4 cycles with zero stitch failures.
CLI mode (invoking `claude` directly on the host) replaced podman, the first production use of the
new execution mode shipped in cobbler-scaffold v0.20260306.3.

## Pre-run Issues

The initial generator:start used `releases: ["04.1"]` (stale from a prior run). Measure proposed
re-implementing ls (rel04.1, already `status: implemented`). We killed the run, updated the config
to `releases: ["02.0"]`, committed the fix, and restarted. Filed cobbler-scaffold issue #847
(planning rule should reject proposals for implemented releases).

## Cycle-by-Cycle Results

Table: Stitch cycles

| Cycle | Task | Title | Status | Duration | Input tokens | Output tokens | Cost USD | Prod LOC delta | Test LOC delta |
|---|---|---|---|---|---|---|---|---|---|
| 2 | #331 | Implement cmd/true and cmd/false | pass | 3m48s | 901,086 | 13,815 | $1.15 | +247 | +140 |
| 3 | #332 | Implement cmd/yes | pass | 1m51s | 877,212 | 5,051 | $0.87 | +69 | +123 |
| 4 | #333 | Implement cmd/basename and cmd/dirname | pass | 4m13s | 1,201,221 | 15,166 | $1.36 | +309 | +201 |

Note: Cycle 1 had no stitch (no open issues at cycle start; measure proposed #331).

## Measure Cycles

Table: Measure cycles

| Cycle | Started | Issues proposed | Input tokens | Output tokens | Cost USD |
|---|---|---|---|---|---|
| 1 (aborted run) | 10:38:38 | 1 (#330, wrong release) | 27,520 | 3,363 | $0.26 |
| 1 (successful) | 10:42:08 | 1 (#331 true/false) | 27,516 | 4,084 | $0.27 |
| 2 | 10:47:46 | 1 (#332 yes) | 27,521 | 2,740 | $0.24 |
| 3 | 10:51:03 | 1 (#333 basename/dirname) | 27,522 | 2,579 | $0.24 |
| 4 | 10:56:31 | 0 (stop) | 27,534 | 1,487 | $0.21 |

## Cost Analysis

Table: Cost summary

| Category | Input tokens | Output tokens | Cost USD |
|---|---|---|---|
| Measure (all 5, incl. aborted) | 137,613 | 14,253 | $1.22 |
| Stitch (3 tasks) | 2,979,519 | 34,032 | $3.38 |
| **Total** | **3,117,132** | **48,285** | **$4.60** |

Total LOC produced: 625 production + 464 test = 1,089 LOC
Cost per KLOC: $4.60 / 1.089 KLOC = **$4.22/KLOC**

Table: Prior run comparison

| Run | Issue | Date | Prod LOC | Test LOC | Total LOC | Cost USD | Cost/KLOC | Notes |
|---|---|---|---|---|---|---|---|---|
| 12 | GH-136 | 2026-01 | — | — | ~2,800 | — | — | eng20 |
| 14 | GH-206 | 2026-03-04 | 2,238 | 3,319 | 5,557 | $59.67 | $10.74 | 8 commands |
| 15 | GH-262 | 2026-03 | — | — | 3,904 | $45.85 | $11.74 | 12 tasks, headers mode |
| **16** | **GH-313** | **2026-03-06** | **625** | **464** | **1,089** | **$4.60** | **$4.22** | CLI mode, trivial utilities |

Run 16 cost/KLOC is 60% lower than runs 14-15, driven by smaller task scope (trivial utilities with
minimal branching logic) and the prompt cache hit rate. Stitch cache_read tokens averaged 932,887 vs
61,596 cache_creation per cycle, yielding ~93% cache utilization.

## Generated Packages

Table: Packages produced

| Package | Files | Prod LOC | Test LOC | Description |
|---|---|---|---|---|
| `pkg/sys` | sys.go | 24 | 0 | SIGPIPE handler, file open helper |
| `pkg/testutils` | testutils.go | 137 | 0 | RunDiffTests differential testing harness |
| `cmd/true` | main.go, true_test.go | 43 | 70 | Exits 0 always |
| `cmd/false` | main.go, false_test.go | 43 | 70 | Exits 1 always |
| `cmd/yes` | main.go, yes_test.go | 69 | 123 | Repeats string indefinitely |
| `cmd/basename` | main.go, basename_test.go | 183 | 108 | Strip directory from path |
| `cmd/dirname` | main.go, dirname_test.go | 126 | 93 | Strip filename from path |

## Failures and Restarts

Table: Issues filed during run

| Issue | Repo | Summary |
|---|---|---|
| cobbler-scaffold #847 | mesh-intelligence/cobbler-scaffold | P1 planning rule does not prevent proposals for releases already marked `status: implemented` in road-map.yaml. Measure proposed re-implementing ls (rel04.1) when `releases` config was stale. |
| cobbler-scaffold #849 | mesh-intelligence/cobbler-scaffold | CLI mode stitch commits show `Loc-Prod-After: 0` in the commit message body even when code was written. The stats YAML files record the correct values; only the commit annotation is wrong. |

No stitch failures in the successful run. All 3 tasks passed on first attempt.

## Observations

CLI mode performed comparably to podman mode in terms of output quality. Prompt caching worked
effectively with 93% cache hit rate. Measure prompt size (~27 KB input) was stable across all
cycles, consistent with the rel02.0 sources being minimal. The measure context stayed well below
the 312 KB scaling wall observed in run 14.

The stitch agent correctly referenced `pkg/testutils` from task #331 in tasks #332 and #333,
demonstrating cross-task context reuse via cache.
