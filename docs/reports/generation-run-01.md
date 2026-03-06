# GH-3 Generation Run Results: Full Codebase from Pipeline

We ran the cobbler pipeline across three generations on 2026-02-25 to produce all Go code for
go-unix-utils releases 01.0, 01.1, and a 02.0 preview. We record the execution metrics, cost
breakdown, and operational lessons from running 16 measure+stitch cycles that produced 4,586
lines of Go across 22 files. The work closed GH-3 via PR #11.

This is the first multi-generation run of the pipeline. Earlier experiments (eng02, eng03)
validated single measure+stitch cycles. This run exercises the full lifecycle: multiple
generations, cross-generation code recovery, and the measure step's ability to identify the
next task from a growing codebase.

## Experiment design

The codebase had 10 PRDs, 7 use cases, 4 test suites, and 0 lines of Go code at the start.
Configuration: temperature 0, Claude Opus 4.6 via Claude Code CLI in Podman containers,
max_time_sec 900, go_source_dirs [pkg, cmd].

We organized the work into three generations, each using the mage generator lifecycle
(generator:start, cobbler:measure, cobbler:stitch cycles, generator:stop):

- Generation A (rel01.0): pkg/testutils differential testing harness, cmd/ts timestamp utility
- Generation B (rel01.1): cmd/cat, cmd/wc, cmd/sponge plus differential tests for all three
- Generation C (rel02.0 preview): pkg/sys system abstractions, pkg/format output formatting

Each generation ran as an independent generator lifecycle. Between generations, we seeded the
new generation branch with existing code from main using git checkout main -- pkg/ cmd/.

## Generation A results (rel01.0)

Generation A ran 4 measure+stitch cycles and produced the foundation: the differential
testing harness and the first command-line utility.

| Cycle | Task | Measure $ | Stitch $ | Stitch time | LOC (prod) | LOC (test) | Turns |
|-------|------|-----------|----------|-------------|------------|------------|-------|
| 1     | pkg/testutils (difftest.go, normalize.go) | $0.607 | $0.964 | 2m22s | 318 | 0 | 17 |
| 2     | cmd/ts (main.go) | $0.570 | $3.066 | 12m8s | 546 | 0 | — |
| Total | — | $1.177 | $4.030 | 14m30s | 864 | 0 | — |

Cycle 1 required two failed stitch attempts before succeeding. The first attempt timed out at
5 minutes (max_time_sec was 300). We increased to 600, and the second attempt timed out at
exactly 10 minutes — the code was written and verified (go build, go vet passed) but the
agent was killed during acceptance criteria verification. We increased to 900, and the third
attempt completed in 2m22s. The timeout was not a model speed problem; earlier runs spent
excessive time in thinking tokens before writing code.

Cycle 2 hit the 32K output token limit mid-response during cmd/ts generation. The stitch
agent recovered and continued, producing a complete 546-line implementation covering strftime
formatting, -i/-s/-m/-r modes, and subsecond extensions.

Two measure cycles were wasted on duplicates. With go_source_dirs initially set to [], the
measure step could not see existing code and proposed pkg/testutils again. Setting
go_source_dirs to [pkg, cmd] resolved this.

## Generation B results (rel01.1)

Generation B ran 8 measure+stitch cycles. The measure step consistently alternated between
implementation and test generation: it would propose a utility, then propose differential
tests for that utility, then move to the next.

| Cycle | Task | Measure $ | Stitch $ | Stitch time | LOC (prod) | LOC (test) | Turns |
|-------|------|-----------|----------|-------------|------------|------------|-------|
| 1     | cmd/ts differential tests | $0.720 | $2.335 | 8m16s | 0 | 215 | — |
| 2     | cmd/cat (main.go) | $0.718 | $3.250 | 8m26s | 357 | 0 | 49 |
| 3     | cmd/cat differential tests | $0.634 | $1.648 | 3m32s | 0 | 253 | 21 |
| 4     | cmd/wc (main.go) | $0.746 | $3.195 | 5m32s | 629 | 0 | 63 |
| 5     | cmd/wc differential tests | $0.779 | $1.332 | 1m29s | 0 | 242 | 11 |
| 6     | cmd/sponge (main.go) | $0.798 | $1.654 | 2m25s | 397 | 0 | 27 |
| 7     | cmd/sponge differential tests | $1.108 | $2.133 | 3m45s | 0 | 443 | 26 |
| Total | — | $5.503 | $15.547 | 33m25s | 1,383 | 1,153 | — |

The measure step saw cmd/ts had no tests (they were generated in Gen A but the Gen B branch
was seeded before ts_test.go existed in Gen A's timeline) and proposed ts tests first. It
then proceeded through cat, wc, and sponge in that order — different from the road-map order
(wc, cat, sponge) but reasonable given the model's assessment of implementation complexity.

cmd/wc was the largest single implementation at 629 LOC, taking 63 turns. cmd/sponge tests
were the largest test file at 443 LOC, reflecting sponge's more complex file-state comparison
testing (it compares output file contents rather than stdout).

## Generation C results (rel02.0 preview)

Generation C ran 4 measure+stitch cycles to produce the two shared library packages needed
for future utilities like ls.

| Cycle | Task | Measure $ | Stitch $ | Stitch time | LOC (prod) | LOC (test) | Turns |
|-------|------|-----------|----------|-------------|------------|------------|-------|
| 1     | pkg/sys (terminal, fileinfo, signal, stat) | $0.997 | $2.732 | 7m12s | 284 | 263 | 47 |
| 2     | pkg/format (columns, color, humansize) | $0.960 | $2.333 | 4m33s | 311 | 328 | 45 |
| Total | — | $1.957 | $5.065 | 11m45s | 595 | 591 | — |

Both packages included tests in the same stitch cycle — a difference from Gen B where measure
proposed implementations and tests separately. The measure step for pkg/sys included
platform-specific files (stat_darwin.go, stat_linux.go) in its proposal. The stitch agent
handled build tags and platform-specific code correctly.

pkg/format generated more test LOC (328) than production LOC (311), reflecting the
table-driven test style with many edge cases for column alignment and ANSI color handling.

## Aggregate metrics

| Metric | Gen A | Gen B | Gen C | Total |
|--------|-------|-------|-------|-------|
| Measure+stitch cycles | 4 | 8 | 4 | 16 |
| Measure cost | $1.18 | $5.50 | $1.96 | $8.64 |
| Stitch cost | $4.03 | $15.55 | $5.07 | $24.65 |
| Total cost | $5.21 | $21.05 | $7.03 | $33.29 |
| Wall clock time | 14m30s | 33m25s | 11m45s | 59m40s |
| Production LOC | 864 | 1,383 | 595 | 2,842 |
| Test LOC | 0 | 1,153 | 591 | 1,744 |
| Total LOC | 864 | 2,536 | 1,186 | 4,586 |
| Files created | 3 | 8 | 11 | 22 |
| Cost per LOC (total) | $6.03 | $8.30 | $5.93 | $7.26 |
| Cost per prod LOC | $6.03 | $15.23 | $11.81 | $11.71 |

The cost per line of code is higher than the eng03 estimate ($0.008/LOC) because that
estimate used a single large task. The multi-cycle approach breaks work into smaller tasks,
each incurring the full measure overhead and stitch context loading. Measure accounts for 26%
of total cost; stitch accounts for 74%.

The per-cycle cost varies from $1.57 (pkg/testutils measure+stitch) to $3.97 (cmd/cat
measure+stitch), driven primarily by stitch turn count. Higher turn counts mean more
accumulated context in the conversation, increasing input token costs.

## Configuration changes during the run

Three configuration parameters required mid-run adjustment:

max_time_sec: Started at 300 (5 minutes), increased to 600, then to 900. The 5-minute and
10-minute limits caused stitch timeouts — the model was writing correct code but was killed
during verification. The 15-minute limit worked for all subsequent cycles, with the longest
stitch taking 12m8s.

go_source_dirs: Started as [] (empty), which caused measure to propose already-built packages
because it could not see existing source files. Changed to [pkg, cmd] to include all Go
source directories in the measure prompt.

Neither change required restarting a generation. Both took effect on the next measure or
stitch invocation.

## Multi-generation lifecycle issues

The generator lifecycle (generator:start, generator:stop) was designed for single-generation
use. Running multiple generations sequentially exposed two problems:

Problem 1: generator:start resets Go sources on the generation branch. Each new generation
begins with zero Go code, so measure cannot see what previous generations built. Workaround:
seed each generation branch with git checkout main -- pkg/ cmd/ after generator:start.

Problem 2: generator:stop resets Go sources on main before merging. The "Prepare main for
generation merge: delete Go code" commit removes all Go sources, and the merge only brings
back files in the generation branch's diff. Code from earlier generations that was seeded
(not generated fresh) is lost. Workaround: after each generator:stop, recover lost files
from earlier generation tags.

Both issues required manual intervention after every generation boundary. We filed
cobbler-scaffold issues #23 and #24 for these.

A third issue (cobbler-scaffold#25): after a stitch timeout, the issue tracker task enters a
state where it cannot be found by the ready query. The workaround is manually resetting the
task status to open.

## Code quality observations

All generated code compiled clean with go build and go vet. The stitch agent ran acceptance
criteria verification for every task, testing each flag and mode individually.

cmd/cat (357 LOC) implements the full POSIX cat with GNU extensions: -n, -b, -s, -v, -e,
-t, -A, -E, -T, and combined flags like -ns. The stitch agent tested squeeze across file
boundaries, binary passthrough with all 256 byte values, and line numbering continuity across
multiple files.

cmd/wc (629 LOC) is the largest utility. It implements -l, -w, -c, -m, -L with proper
multibyte character counting via unicode/utf8. The high turn count (63) reflects the agent
testing each flag combination and edge case.

cmd/sponge (397 LOC) implements the soak-before-write contract with atomic rename, temp-file
spill for large inputs, append mode, and permission preservation. The differential tests
(443 LOC) use file-state comparison rather than stdout comparison, correctly implementing
prd001-testutils R5.

pkg/sys (284 LOC + 263 LOC tests) includes platform-specific stat implementations for
darwin and linux, terminal width detection via ioctl, and signal handling with context
cancellation.

pkg/format (311 LOC + 328 LOC tests) implements column alignment with variable-width fields,
16-color and 256-color ANSI sequences, and human-readable size formatting (Ki/Mi/Gi).

All test files skip gracefully when reference binaries are not available on PATH, using
t.Skip rather than t.Fatal.

## Measure step behavior patterns

The measure step exhibited consistent patterns across all three generations:

Pattern 1 (test-before-next): When measure sees existing implementation code without tests,
it proposes tests before moving to new implementations. This happened with cmd/ts in Gen B
and was consistent across all utilities.

Pattern 2 (model-chosen ordering): Measure does not follow the road-map order. In Gen B, it
chose cat before wc (road-map lists wc first). The ordering reflects the model's assessment
of dependency structure and implementation complexity, not the release plan.

Pattern 3 (implementation-then-tests in shared packages): For pkg/sys and pkg/format (Gen C),
measure proposed both implementation and tests in a single task. For cmd/ utilities (Gen B),
it separated them. The difference may be that shared packages have simpler testing patterns
(unit tests) while cmd/ utilities need differential tests with reference binary lookup.

Pattern 4 (stable cost): Measure cost remained between $0.57 and $1.11 across all 16 cycles,
with no correlation to codebase size. The prompt grows with each added source file, but
cache reads absorb most of the cost increase.

## References

- `.cobbler/history/` — All measure and stitch prompts, logs, stats, and reports
- `docs/engineering/eng02-measure-reproducibility.yaml` — GH-5 reproducibility analysis
- `docs/engineering/eng03-pipeline-validation.yaml` — Single-cycle pipeline validation
- https://github.com/petar-djukic/go-unix-utils/pull/11 — PR #11: GH-3 code generation
- https://github.com/petar-djukic/cobbler-scaffold/issues/23 — generator:stop destroys earlier code
- https://github.com/petar-djukic/cobbler-scaffold/issues/24 — generator:start should seed existing code
- https://github.com/petar-djukic/cobbler-scaffold/issues/25 — issue tracker ready query stale after stitch timeout
