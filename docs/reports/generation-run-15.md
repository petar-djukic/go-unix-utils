# GH-262 Generation Run 15: 12 Tasks, 3,904 LOC, $45.85, Headers-Only Measure Mode

We ran the cobbler pipeline on generation branch generation-gh-262-codegen targeting seven
releases (rel00.0 through rel04.1) across two sessions (2026-03-04 to 2026-03-06). The run
stitched 12 tasks, producing 2,484 production LOC and 1,420 test LOC at a total cost of
$45.85 ($11.74/KLOC).

This was the first run using measure_source_mode: headers, which holds the measure prompt
at ~70 KB regardless of codebase size by substituting full source files with function-signature
summaries. The prompt scaling wall that halted run 14 (312 KB, 4 watchdog kills) was eliminated:
all 3 failures in this run were transient rate-limit stalls, not prompt-size-driven.

Stitch cost dominated at $34.15 (74%) vs. measure at $11.70 (26%), consistent with prior runs.
Of 22 stitch attempts, 7 failed (32% failure rate) due to rate-limit stalls and one timeout,
requiring 3 manual restarts via mage generator:resume.

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Generation branch | generation-gh-262-codegen |
| Scaffold version | v0.20260304.0 |
| Model | Claude Opus 4.6 (in podman containers) |
| Temperature | 0 |
| Max time per agent | 900s (15 min) |
| Max measure issues | 1 |
| Max stitch issues per cycle | 1 |
| CLAUDE_CODE_MAX_OUTPUT_TOKENS | 128,000 |
| max_requirements_per_task | 4 |
| idle_timeout_seconds | 900 |
| measure_source_mode | headers |
| measure_roadmap_source | true |
| releases | ["00.0"] → advanced after each stitch to ["04.1"] |
| Seeded LOC | 0 (clean start) |
| Budgeted cycles | unlimited (cycles: 0) |
| Successful stitch tasks | 12 |
| Rate-limit interruptions | 3 |
| Manual restarts | 3 |

## Cycle-by-cycle results

Table 2: All successful stitch tasks in execution order

| # | Issue | Description | Stitch Time | Input Tokens | Output Tokens | Cost | LOC +Prod | LOC +Test |
|---|-------|-------------|-------------|--------------|---------------|------|-----------|-----------|
| 1 | #280 | pkg/format shared formatting library (rel00.0-uc002) | 14m17s | 2,140,710 | 52,701 | $2.88 | +284 | +383 |
| 2 | #282 | pkg/testutils DiffTest differential testing harness (rel00.0-uc004) | 11m23s | 1,118,741 | 40,569 | $1.83 | +135 | +127 |
| 3 | #287 | pkg/testutils (follow-up cleanup) | 2m0s | 368,894 | 6,854 | $0.64 | 0 | 0 |
| 4 | #289 | pkg/sys syscall abstractions (rel00.0-uc003) | 13m59s | 53,775 | 486 | $0.27 | 0 | 0 |
| 5 | #290 | magefiles build lifecycle and cmd/version binary (rel00.0-uc001) | 13m10s | 5,992,101 | 40,301 | $4.51 | +39 | 0 |
| 6 | #291 | pkg/format column alignment, ANSI color, size formatting | 3m3s | 315,098 | 7,892 | $0.66 | 0 | 0 |
| 7 | #293 | cmd/cat file concatenation and output transformation (prd-cat R1-R4) | 11m2s | 1,801,240 | 39,404 | $2.39 | +267 | 0 |
| 8 | #295 | pkg/sys syscall abstractions retry — rel00.0-uc003 | 6m9s | 1,767,145 | 22,789 | $1.87 | +232 | +177 |
| 9 | #297 | cmd/cat differential tests (rel01.1-uc001-cat) | 7m5s | 621,754 | 26,002 | $1.34 | 0 | +350 |
| 10 | #299 | cmd/sponge main.go and differential tests (rel01.2-uc001) | 10m25s | 1,787,517 | 38,344 | $2.78 | +196 | +383 |
| 11 | #304 | cmd/du core disk usage traversal and reporting (rel01.3-uc001) | 14m37s | 2,435,259 | 51,218 | $3.68 | +312 | 0 |
| 12 | #306 | cmd/wc core counting (rel03.0-uc001) | 6m45s | 5,135,655 | 17,578 | $3.59 | +463 | 0 |
| 13 | #308 | cmd/ls core — directory reading, filtering, sorting, output | 10m8s | 2,488,838 | 33,447 | $3.26 | +374 | 0 |
| 14 | #310 | cmd/ls extended — sort modes, time display, inode/block metadata | 10m16s | 2,177,761 | 33,181 | $2.77 | +182 | 0 |

Note: 7 stitch attempts failed (issues #282×3, #293×1, #299×3) due to rate-limit stalls.
The failed #299 attempts were caused by Claude attempting to apt-install moreutils.

## Measure cycles

Table 3: All measure cycles in execution order

| Timestamp (UTC) | Duration | Status | Input Tokens | Output Tokens | Cache Read | Cost | Prod LOC at start |
|-----------------|----------|--------|--------------|---------------|------------|------|-------------------|
| 2026-03-04T22:47 | 4m21s | success | 35,190 | 16,120 | 13,920 | $0.54 | 0 |
| 2026-03-04T22:53 | 4m23s | success | 35,041 | 15,450 | 15,967 | $0.51 | 0 |
| 2026-03-05T00:15 | 6m1s | success | 35,041 | 20,790 | 15,967 | $0.65 | 284 |
| 2026-03-05T00:31 | 4m20s | success | 35,092 | 15,698 | 19,189 | $0.50 | 284 |
| 2026-03-05T00:50 | 1s | FAIL | 0 | 0 | 0 | $0.00 | 284 |
| 2026-03-05T13:03 | 4m56s | success | 35,092 | 17,827 | 15,967 | $0.57 | 284 |
| 2026-03-05T13:22 | 5m12s | success | 35,039 | 18,994 | 19,189 | $0.58 | 419 |
| 2026-03-05T13:35 | 5m25s | success | 35,037 | 19,828 | 35,035 | $0.51 | 419 |
| 2026-03-05T17:32 | 4m58s | success | 35,085 | 18,171 | 15,967 | $0.58 | 419 |
| 2026-03-05T17:51 | 4m44s | success | 35,091 | 16,918 | 19,189 | $0.53 | 458 |
| 2026-03-05T18:00 | 5m10s | success | 35,031 | 19,614 | 19,189 | $0.60 | 458 |
| 2026-03-05T18:13 | 1s | FAIL | 0 | 0 | 0 | $0.00 | 458 |
| 2026-03-05T22:42 | 2m48s | success | 35,079 | 10,189 | 15,967 | $0.38 | 725 |
| 2026-03-05T22:51 | 2m44s | success | 35,079 | 9,264 | 19,189 | $0.34 | 957 |
| 2026-03-05T23:02 | 2m55s | success | 35,031 | 10,962 | 19,189 | $0.38 | 957 |
| 2026-03-05T23:06 | 1m46s | success | 35,027 | 6,294 | 19,189 | $0.27 | 957 |
| 2026-03-05T23:08 | 1s | FAIL | 0 | 0 | 0 | $0.00 | 957 |
| 2026-03-06T03:19 | 3m12s | success | 35,175 | 9,129 | 16,066 | $0.36 | 957 |
| 2026-03-06T03:34 | 2m24s | success | 35,124 | 7,017 | 16,066 | $0.30 | 1,153 |
| 2026-03-06T03:37 | 5m6s | success | 35,120 | 15,547 | 19,286 | $0.50 | 1,153 |
| 2026-03-06T03:57 | 5m26s | success | 35,120 | 17,286 | 16,066 | $0.56 | 1,465 |
| 2026-03-06T04:03 | 3m50s | success | 35,116 | 12,531 | 14,011 | $0.45 | 1,465 |
| 2026-03-06T04:14 | 3m15s | success | 35,116 | 10,251 | 16,066 | $0.38 | 1,928 |
| 2026-03-06T04:18 | 4m58s | success | 35,112 | 15,914 | 19,286 | $0.51 | 1,928 |
| 2026-03-06T04:35 | 4m46s | success | 35,112 | 14,946 | 16,066 | $0.50 | 2,302 |
| 2026-03-06T04:40 | 6m54s | success | 35,108 | 20,663 | 14,011 | $0.66 | 2,302 |
| 2026-03-06T04:58 | 900s | FAIL (watchdog) | 0 | 0 | 0 | $0.00 | 2,484 |

Measure input tokens are remarkably stable: 35,000–35,200 across all 27 cycles. This
confirms that headers mode eliminates the linear prompt growth observed in run 14. The
variation is from road-map and context file changes, not source code growth.

## Cost analysis

Table 4: Cost breakdown

| Category | Cycles | Successful | Cost |
|----------|--------|------------|------|
| Measure | 27 | 24 | $11.70 |
| Stitch | 22 | 15 | $34.15 |
| Total | 49 | 39 | $45.85 |

At $45.85 for 3,904 LOC, the cost is $11.74 per 1,000 LOC.

Table 5: Cost per KLOC comparison across runs

| Run | Model | Total LOC | Cost | Cost/KLOC |
|-----|-------|-----------|------|-----------|
| 11 (eng18) | Sonnet 4.6 | 4,218 | ~$23 | ~$5.46 |
| 12 (eng20) | Opus 4.6 | 5,303 | ~$36 | ~$6.79 |
| 14 (eng21) | Opus 4.6 | 5,557 | $59.67 | $10.74 |
| 15 (eng24) | Opus 4.6 | 3,904 | $45.85 | $11.74 |

Run 15 cost/KLOC is slightly higher than run 14. The headers optimization reduced measure
cost (stable ~35K input tokens vs. growing 70-312K in run 14), but run 15 had a higher
stitch failure rate (32% vs. ~10% in run 14), inflating total cost with wasted stitch
tokens. The three sponge retries alone cost ~$0.73 in failed attempts.

Table 6: Token totals

| Category | Input Tokens | Output Tokens | Cache Creation | Cache Read | Cost |
|----------|-------------|----------------|---------------|------------|------|
| Measure | 726,254 | 393,527 | — | 436,353 | $11.70 |
| Stitch | 26,633,379 | 510,375 | 1,128,298 | 24,405,128 | $34.15 |
| Total | 27,359,633 | 903,902 | 1,128,298 | 24,841,481 | $45.85 |

Stitch input tokens are dominated by cache reads (24.4M of 26.6M total). The high cache
utilization is expected: each stitch cycle loads the full codebase context, which is
stable across invocations.

## Generated packages

Table 7: Packages produced in run 15

| Package | Prod LOC | Test LOC | Release |
|---------|----------|----------|---------|
| pkg/format | 284 | 383 | rel00.0 |
| pkg/testutils | 135 | 127 | rel00.0 |
| pkg/sys | 232 | 177 | rel00.0 |
| cmd/version | 39 | 0 | rel00.0 |
| cmd/cat | 267 | 350 | rel01.1 |
| cmd/sponge | 196 | 383 | rel01.2 |
| cmd/du | 312 | 0 | rel01.3 |
| cmd/wc | 463 | 0 | rel03.0 |
| cmd/ls (core + extended) | 556 | 0 | rel04.0, rel04.1 |
| Total | 2,484 | 1,420 | |

LOC counts derived from loc_before/loc_after deltas in stitch-stats.yaml files.

## Prompt size stability (headers mode)

Measure input tokens held at 35,000–35,200 across all 27 cycles regardless of codebase
size growth from 0 to 2,484 production LOC. This is a 9× reduction from run 14's
endpoint (312 KB ≈ 311,000 tokens). The headers mode replaces Go source files with
function-signature summaries in the measure prompt.

Table 8: Prompt size comparison

| Run | Mode | Start Tokens | End Tokens | Prompt-driven failures |
|-----|------|-------------|------------|----------------------|
| 14 (eng21) | full source | ~70,000 | ~311,000 | 4 |
| 15 (eng24) | headers | ~35,000 | ~35,200 | 0 |

The 3 measure failures in run 15 were all rate-limit stalls (0 tokens in/out), not
prompt-size-driven. The practical scaling ceiling has been removed.

## Failures and restarts

Table 9: Failures during run 15

| Time (UTC) | Type | Cause | Cost Wasted | Resolution |
|------------|------|-------|-------------|------------|
| 2026-03-05T00:50 | measure | Rate-limit stall (1s, 0 tokens) | $0.00 | Auto-retry |
| 2026-03-05T00:21 | stitch #282 | Rate-limit stall | $0.00 | mage generator:resume |
| 2026-03-05T00:36 | stitch #282 | Rate-limit stall | $0.00 | mage generator:resume |
| 2026-03-05T12:50 | stitch #282 | Rate-limit stall | $0.00 | mage generator:resume |
| 2026-03-05T18:06 | stitch #293 | Rate-limit stall | $1.13 | mage generator:resume |
| 2026-03-05T18:13 | measure | Rate-limit stall (1s, 0 tokens) | $0.00 | Auto-retry |
| 2026-03-05T23:07 | stitch #299 | Claude attempted apt install of moreutils | $0.36 | Issue body patched |
| 2026-03-06T02:28 | stitch #299 | Claude attempted apt install of moreutils | $0.37 | Issue body patched |
| 2026-03-05T23:08 | measure | Rate-limit stall (1s, 0 tokens) | $0.00 | Auto-retry |
| 2026-03-06T03:04 | stitch #299 | Rate-limit stall (900s watchdog) | $0.00 | mage generator:resume |
| 2026-03-06T04:58 | measure | Rate-limit stall (900s watchdog, final) | $0.00 | Run closed out |

The sponge (#299) failures are notable: the first two were caused by Claude attempting
to install the moreutils reference binary via apt/apk. The issue body was patched after
each attempt to add explicit prohibitions. The third attempt was a rate-limit stall.
Total wasted cost: ~$1.86 across all failed attempts.

## Duplicate measure pattern

After each successful stitch, measure proposed a duplicate issue for the just-completed
release because road-map.yaml still shows spec_complete at measure time. The workaround
required closing the duplicate, marking the release implemented in road-map.yaml, advancing
the releases list in configuration.yaml, and committing — approximately 2 minutes of manual
overhead per release (7 duplicates total across this run).

cobbler-scaffold v0.20260305.3 (PR #314) added planning constitution rule P1 to prevent
this in future runs: "Do not propose any new tasks for use cases with status 'implemented'
or 'done'."

## Recommendations

1. headers mode is stable. All future runs should use measure_source_mode: headers.
   The prompt size scaling wall from run 14 is eliminated.

2. Stitch failure rate (32%) is too high. Three sponge failures wasted ~$1.86. Investigate
   whether the podman container environment can be configured to avoid rate-limit stalls
   by pre-emptively backing off when the API returns errors.

3. planning constitution rule P1 (cobbler-scaffold v0.20260305.3) should eliminate the
   duplicate measure pattern. Verify in run 16.

4. The final measure cycle for ts (rel99.2) timed out after 900s with 0 tokens. ts has
   now failed to start in multiple consecutive runs. Consider pre-seeding the ts issue
   body rather than relying on measure to propose it.

## References

- docs/engineering/eng21-generation-run-14-results.yaml — Previous run results
- https://github.com/petar-djukic/go-unix-utils/issues/262 — GH-262: Generation run 15
- generation-gh-262-codegen-merged — Generation tag with full .cobbler/history/
- https://github.com/petar-djukic/go-unix-utils/pull/312 — PR #312: Generation run 15 close-out
- https://github.com/petar-djukic/cobbler-scaffold/issues/747 — Bug: orphaned measuring placeholders on Claude failure
