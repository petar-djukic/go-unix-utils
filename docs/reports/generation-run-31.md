# Generation Run 31

Date: 2026-03-10 to 2026-03-14
Issue: GH-799
Branch: generation-gh-799-run31
Tag: generation-gh-799-run31-finished
Scaffold: cobbler-scaffold v0.20260310.0

## Summary

Run 31 generated 6 commands across 2 releases (rel07.0, rel07.1) in addition to completing
the remaining work from rel05.4 through rel06.1. The run produced 17,651 production and
26,969 test LOC across 152 files, with 241 successful stitch tasks and 22 failures. Total
cost was $139.24 ($53.96 stitch + $85.27 measure). The generation consumed 46,343 lines of
insertions and 1,717 lines of deletions across all task commits (3.7% churn rate).

This was the largest single generation run to date, producing 44,628 net LOC at
$0.0053/LOC.

## Results

| Release | Commands | Tasks | Prod LOC | Test LOC |
|---------|----------|-------|----------|----------|
| rel05.4 | sha256sum, sha512sum | included in total | | |
| rel05.5 | ts | included in total | | |
| rel06.0 | mkdir, rmdir, ln, mktemp | included in total | | |
| rel06.1 | env, printenv, id, whoami, groups | included in total | | |
| rel07.0 | sort, tr, tail | 13 | +2,072 | +1,880 |
| rel07.1 | cp, mv, rm | 12 | +1,587 | +4,091 |

Per-PRD breakdown (rel07.0 and rel07.1 only):

| PRD | Tasks | Cost | Turns | Insertions | Deletions | Net LOC |
|-----|-------|------|-------|------------|-----------|---------|
| prd053-sort | 7 | $10.23 | 238 | 1,895 | 46 | 1,849 |
| prd054-tr | 4 | $5.17 | 123 | 1,406 | 49 | 1,357 |
| prd055-tail | 4 | $4.38 | 121 | 945 | 21 | 924 |
| prd056-cp | 4 | $6.04 | 156 | 1,895 | 38 | 1,857 |
| prd057-mv | 4 | $5.94 | 111 | 1,785 | 16 | 1,769 |
| prd058-rm | 4 | $5.55 | 83 | 1,956 | 26 | 1,930 |

862 of 867 R-items addressed (99%). 5 remaining are prd011-magefiles (manual, not
generatable).

## Code Deletion Analysis

Across all 241 successful stitch tasks, Claude inserted 46,343 lines and deleted 1,717
lines (3.7% churn). The net LOC delivered to the branch was 44,626, meaning 1,717 lines
were written and then removed during the generation.

### Why deletions happen

Deletions fall into three categories:

1. **Refactoring during incremental builds (majority)**: When a later task in the same
   command adds flags or modes, Claude restructures existing code to accommodate the new
   feature. For example, sort R4.1-R4.4 (task #1252, 12 deletions) rewrote argument
   parsing to add check mode. This is correct behavior — the code should evolve as
   requirements are added incrementally.

2. **Fix-compile-retry cycles**: Claude writes code, runs `go build`, gets a compile
   error, deletes the broken code, and rewrites it. The deletions represent wasted first
   attempts. Tasks with high turn counts (40+) typically exhibit this pattern. 27 tasks
   (11%) had 40+ turns.

3. **Test refactoring**: Later tasks restructure test helpers or fixtures written by
   earlier tasks. For example, the unexpand task (#1080) deleted 79 lines of test
   scaffolding that had been written during the expand task, replacing it with a more
   general structure.

### Highest-deletion tasks

| Task | Del | Ins | Turns | Cost | Command |
|------|-----|-----|-------|------|---------|
| #1080 | 79 | 382 | 14 | $1.52 | unexpand R2 (refactored expand test helpers) |
| #811 | 65 | 23 | 20 | $0.96 | testutils R5 (WorkDir rewrite) |
| #982 | 49 | 727 | 40 | $1.62 | ls R1.5-R1.8 (large initial implementation) |
| #1058 | 46 | 288 | 23 | $0.86 | nl R3 (number line restructure) |
| #1162 | 39 | 283 | 61 | $3.35 | ts R6.1-R6.4 (timestamp format rewrite) |
| #874 | 39 | 127 | 49 | $2.43 | du R2.8-R3.3 (filtering rewrite) |

## Claude Code Inefficiency Analysis

### Compilation overhead

Claude invoked Go tooling 869 times across 241 tasks:

| Command | Count | Per task |
|---------|-------|----------|
| go build | 363 | 1.5 |
| go test | 385 | 1.6 |
| go vet | 114 | 0.5 |
| **Total** | **862** | **3.6** |

The ideal minimum is 2 Go invocations per task (1 build + 1 test). The actual average of
3.6 means Claude runs 1.8x more Go commands than necessary. 15 tasks had 3+ build
invocations, indicating write-compile-fix-compile loops.

### High-turn tasks

27 tasks (11%) consumed 40+ turns. These tasks account for a disproportionate share of
cost and time:

| Task | Turns | Cost | Duration | Ins | Del | Command |
|------|-------|------|----------|-----|-----|---------|
| #1252 | 104 | $3.48 | 5m53s | 232 | 12 | sort R4 (4 builds, test retries) |
| #986 | 80 | $4.02 | 13m49s | 354 | 10 | ls R1.13-R2.2 (rate limit + retries) |
| #996 | 67 | $2.85 | 7m50s | 259 | 15 | ls R3.4-R3.7 |
| #1002 | 63 | $2.39 | 8m5s | 164 | 3 | ls R4.1-R4.4 |

The sort R4 task (#1252) is instructive: 104 turns for 232 net lines. Claude used the
extra turns for (a) reading reference implementations of GNU sort to match behavior,
(b) 4 separate `go build` cycles to fix compile errors, (c) running individual test
cases to debug mismatches, and (d) comparing output with `gsort` byte-by-byte. The
productive work (reading spec, writing code) was roughly 30 turns; the remaining 74
were verification and retry loops.

### Pattern: read-write-compile-fix-compile

Claude's approach to each task follows a consistent but wasteful pattern:

1. Read spec (2-4 turns)
2. Read existing code (2-4 turns)
3. Write implementation (5-15 turns of individual Edit calls)
4. Run `go build` (1 turn)
5. Fix compile errors (1-3 turns of Edit + build)
6. Run `go test` (1 turn)
7. Fix test failures (2-10 turns of Edit + test)
8. Run final `go test` to confirm (1 turn)

Steps 5 and 7 are where waste concentrates. Claude does not mentally compile the code
before writing it — it relies on the Go compiler as a feedback loop. A better approach
would batch all edits and compile once, treating the compiler as a final check rather
than an interactive debugger.

### Specific inefficiencies observed

- **Using `cat`, `sed`, `xxd` instead of Read/Edit tools**: Task #1252 used `cat -A` and
  `xxd` to inspect files, and `sed -i` to edit, burning turns on tool misuse when the
  dedicated Read and Edit tools are available.

- **Comparing with reference implementations at runtime**: Many tasks ran both the
  generated binary and the GNU reference (`gsort`, `gls`, etc.) to compare output. This
  is useful for differential testing but burns turns when the test suite already covers
  the behavior. The stitch prompt could instruct Claude to trust the test suite rather
  than re-verifying manually.

- **Individual Edit calls per function**: Claude makes one Edit tool call per function or
  code block. A file with 5 functions gets 5+ Edit calls when a single Write with the
  complete file would be faster and avoid the risk of failed edits due to context
  mismatches.

## Failures and Recovery

22 of 263 stitch invocations failed (8.4%). Most failures cost $0 (recovered in <5s):

| Failure type | Count | Wasted cost |
|--------------|-------|-------------|
| Rate limit watchdog kill | 6 | $0.97 |
| GitHub API connectivity loss | 5 | $0 |
| Stale label recovery | 8 | $0 |
| Actual task failure | 3 | $1.27 |
| **Total** | **22** | **$2.24** |

307 rate limit events were recorded across all stitch sessions. Most were minor (Claude
waited and resumed). 6 events caused idle watchdog kills where Claude stalled for 15-27
minutes, triggering the idle timeout. Recovery required manual label cleanup
(`cobbler-in-progress` -> `cobbler-ready`) and generator restart.

## Measure Overhead

252 measure invocations at $85.27 (61% of total cost). Each measure call costs ~$0.34
and produces 1 task proposal. This is the dominant cost driver — measure costs 1.6x more
than stitch despite doing no code generation.

Measure token consumption grew from 48K to 56K input tokens per call as the requirements
file expanded. Output stayed small (1-8K tokens). The 2-turn measure pattern (read
context, propose task) is efficient per-call but the sheer number of invocations (252 for
241 tasks) adds up.

Measure proposed 11 tasks that were immediately rejected by programmatic validation
(requirement count outside P9 range, P7 violation for test file naming). These proposals
were accepted anyway (warnings, not errors) but indicate measure does not fully internalize
the stitch constraints.

## Issues Filed

| Issue | Title | Repo | Status |
|-------|-------|------|--------|
| cobbler-scaffold#1449 | stats:generator per-task LOC delta incorrect | cobbler-scaffold | open |
| cobbler-scaffold#1450 | stats:generator uses GitHub API without pagination | cobbler-scaffold | open |
| cobbler-scaffold#1451 | Measure proposes tasks for non-generatable requirements | cobbler-scaffold | open |

## Metrics

| Metric | Value |
|--------|-------|
| Calendar time | 4 days |
| Stitch wall time | 12.8h |
| Cycles | ~42 (7 batches) |
| Measure invocations | 252 |
| Stitch tasks (success) | 241 |
| Stitch tasks (failed) | 22 |
| Total cost | $139.24 |
| Stitch cost | $53.96 |
| Measure cost | $85.27 |
| Wasted cost (failures) | $2.24 |
| Total turns | 5,764 (stitch) + 472 (measure) |
| Tokens in | 58.1M |
| Tokens out | 1.3M |
| Production LOC | 17,651 |
| Test LOC | 26,969 |
| Total LOC | 44,620 |
| Net LOC (git diff) | 44,628 |
| Files changed | 152 |
| Total insertions | 46,343 |
| Total deletions | 1,717 |
| Churn rate | 3.7% |
| Cost per net LOC | $0.0053 |
| Go build invocations | 363 |
| Go test invocations | 385 |
| Go vet invocations | 114 |
| Go invocations per task | 3.6 |
| Rate limit events | 307 |
| R-items complete | 862 / 867 (99%) |
| R-items remaining | 5 (prd011-magefiles, manual) |
| Releases completed | rel00.0 through rel07.1 |
| PRDs fully addressed | 58 / 59 |

## Comparison

| | Run 28 | Run 31 |
|---|--------|--------|
| Scaffold | v0.20260309.4 | v0.20260310.0 |
| Stitch tasks | 47 | 241 |
| Failed tasks | - | 22 (8.4%) |
| Total cost | $57.38 | $139.24 |
| LOC | 5,841 | 44,620 |
| Cost per LOC | $0.0098 | $0.0053 |
| Releases | rel00.0-rel02.0 (partial) | rel00.0-rel07.1 |
| Churn (del/ins) | - | 3.7% |

Run 31 was 1.8x more cost-efficient than run 28 per LOC. The improvement comes from
reduced prompt size (cmd/ removed from go_source_dirs, tests excluded from stitch context)
and the elimination of the duplicate task problem.

## Recommendations

### Reduce measure cost (61% of total)

Measure accounts for $85.27 of $139.24. Each invocation reads the full requirements state
and road-map, proposes 1 task, and exits. Batching multiple task proposals per measure call
(e.g. 3-5 tasks) would cut measure invocations from 252 to ~50-80, saving ~$60.

### Reduce Claude's compile-retry cycles

The 3.6 Go invocations per task (vs ideal 2.0) represents a 80% overhead. Two approaches:

1. **Pre-compilation lint in the stitch prompt**: Instruct Claude to mentally trace
   imports, types, and function signatures before writing code. "Do not run `go build`
   until all edits are complete."

2. **Batch Edit tool**: Allow Claude to submit multiple edits in a single turn, reducing
   the read-edit-compile-edit-compile loop to read-edit-compile.

### Address non-generatable requirements

cobbler-scaffold#1451 proposes a `skip` status for requirements that cannot be generated
(like prd011-magefiles). Without this, the generator will hit zero-LOC loops whenever
only manual requirements remain in scope.

### Reduce reference implementation comparison

Claude spent significant turns running GNU reference binaries (`gsort`, `gls`, etc.)
alongside the generated binary. The stitch prompt should clarify that the test suite
is the acceptance gate, not manual comparison. Reference comparison should happen only
when a test fails unexpectedly.
