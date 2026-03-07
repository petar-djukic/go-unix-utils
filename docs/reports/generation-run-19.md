# GH-399 Generation Run 19: Full Regeneration from Specifications

We ran generation run 19 to regenerate the entire codebase from specifications. All 16
releases (rel00.0 through rel99.2) were reset from `implemented` to `spec_complete` and
regenerated from scratch using cobbler-scaffold CLI mode. The run produced 27 utilities
across 77 files, totaling 15,768 lines of code (9,686 production + 6,077 test).

## Configuration

Table 1: Generation parameters

| Parameter | Value |
|-----------|-------|
| Baseline tag | generation-gh-399-generate-code-start |
| Generation branch | generation-gh-399-generate-code |
| Model | Claude Opus 4.6 (CLI mode) |
| Mode | cli |
| Max time per agent | 900s (15 min) |
| Estimated lines | 300-500 |
| Context mode | headers (measure_source_mode=headers) |
| Releases targeted | 00.0, 01.1, 01.2, 01.3, 02.0, 02.1, 02.2, 03.0, 04.0, 04.1, 05.0, 05.1, 05.2, 05.3, 05.4, 99.2 |

## Per-release results

Table 2: Release-by-release metrics

| Release | Utilities | Tasks | Prod LOC | Test LOC | Notes |
|---------|-----------|-------|----------|----------|-------|
| rel00.0 | pkg/sys, pkg/testutils, pkg/format, cmd/version | #407-#412 | 853 | 1,435 | 3 duplicate prd002-sys tasks (#413, #416, #417) produced 0 LOC |
| rel01.1 | cat | #419, #420 | +182 | +195 | |
| rel01.2 | sponge | #422, #423 | +91 | +193 | |
| rel01.3 | du | #425, #426 | +344 | +263 | #425 timed out on first attempt (15m, 0 tokens); succeeded on retry in 7m41s |
| rel02.0 | true, false, yes, basename, dirname | #429-#432 | +834 | +392 | true+false combined in one task (#429, 62 lines) |
| rel02.1 | tee | #434 | +139 | +155 | Single task, impl + tests together |
| rel02.2 | head, seq | #436, #437 | +770 | +379 | |
| rel03.0 | wc | #439 | +322 | +204 | 11m16s, close to 15m limit |
| rel04.0 | ls core | #441, #442 | +759 | +536 | R1-R4 basic + R5-R8 advanced |
| rel04.1 | ls extended | #444-#446 | +506 | +122 | 3 tasks: R9-R12 (13m51s), R13-R16, R17-R20 |
| rel05.0 | echo, tac, nl, fold | #448-#451 | +1,200 | +438 | nl was largest at 544+184 lines |
| rel05.1 | expand, unexpand | #453, #454 | +565 | +331 | |
| rel05.2 | cut, paste | #456, #457 | +922 | +480 | |
| rel05.3 | uniq, comm | #459, #460, #462 | +791 | +387 | #459 failed 4x with "Prompt is too long" before go_source_dirs fix |
| rel05.4 | md5sum, sha1sum, sha256sum, sha512sum | #464-#467 | +1,492 | +796 | 4 structurally identical checksum utilities |
| rel99.2 | ts | #469 | +262 | +0 | Tests included in main.go task |

## Aggregate results

Table 3: Run totals

| Metric | Value |
|--------|-------|
| Total production LOC | 9,686 |
| Total test LOC | 6,287 |
| Total LOC | 15,973 |
| Files produced | 79 |
| Utilities implemented | 27 |
| Successful stitch tasks | ~35 |
| Failed stitch tasks | 5 (1 timeout, 4 prompt-too-long) |
| Measure cycles | ~35 |
| Duplicate/empty measures | ~16 (one per release transition) |
| Releases completed | 16 of 16 |
| Generation tag | generation-gh-399-generate-code-finished |
| PR | #471 |

## Prompt-too-long incident

At rel05.3 (~12K LOC), the stitch prompt exceeded the 200K token context window. The
stitch prompt bundles all source files from `go_source_dirs` (which included both `pkg/`
and `cmd/`). With 22 cmd/ directories and all pkg/ libraries, the prompt reached 583KB
(~199K tokens).

Task #459 (cmd/uniq) failed 4 times with `error: 'claude failure: exit status 1'` and
the Claude log showed `Prompt is too long`. The fix was removing `cmd` from `go_source_dirs`
in configuration.yaml, reducing the stitch prompt to ~95KB. In CLI mode, the stitch agent
reads cmd/ files on demand via filesystem access, so excluding them from the bundled prompt
has no functional impact.

This is a cobbler-scaffold limitation. The stitch prompt should dynamically include only
directories relevant to the current task (derivable from the PRD's directory list) rather
than all directories in `go_source_dirs`. Filed as a cobbler-scaffold improvement.

## Timeout incident

Task #425 (cmd/du R1-R4 implementation) timed out on its first attempt after 15 minutes
with 0 input/output tokens. The Claude process appears to have hung before producing any
output. On retry, the same task succeeded in 7m41s with normal token counts.

## Measure-duplicate pattern

After each release completed, the measure agent proposed tasks for already-implemented code
because road-map.yaml still showed `spec_complete`. The fix cycle was manual each time:

1. Close the duplicate issue (or let the empty measure return `[]`)
2. Mark the release and its use cases as `implemented` in road-map.yaml
3. Advance the `releases` field in configuration.yaml
4. Commit and resume

This added ~2 minutes of manual overhead per release (16 releases = ~32 minutes total).
A cobbler-scaffold improvement to auto-advance releases after all tasks close would
eliminate this overhead.

## Post-generation: missing test files (GH-473)

After the generation run, cmd/echo and cmd/ts were missing differential test files. The
generation produced main.go for both utilities but no _test.go files. GH-473 added them
manually:

Table 5: Post-generation test additions

| Utility | File | Test LOC | Tests | Reference binary |
|---------|------|----------|-------|------------------|
| echo | cmd/echo/echo_test.go | 68 | 8 differential | gecho |
| ts | cmd/ts/ts_test.go | 142 | 9 differential + 2 error | ts (moreutils) |

The ts error cases (invalid flag, mutually exclusive -i/-s) are tested standalone rather
than differentially because the reference ts (Perl) exits 255 while Go exits 1, and the
stderr message formats differ. A custom `subsecondNormalizer` was added for `%.S` format
since `TimestampNormalizer` only covers `HH:MM:SS` patterns. R6 (`-r` relative mode) tests
were omitted because the Go implementation does not support `-r`.

The road map was also updated to promote ts from deferred rel99.2 to rel05.5.

Updated totals: 15,973 LOC (9,686 production + 6,287 test), 79 files.

## Comparison with previous runs

Table 4: Generation runs at scale

| Run | Releases | Utilities | LOC | Cost/KLOC | Mode |
|-----|----------|-----------|-----|-----------|------|
| Run 17 (GH-336) | 11 (rel02.1-05.4) | 17 | 7,194 | $5.41 | CLI |
| Run 18 (GH-380) | 1 (rel99.2) | 1 | 565 | $19.75 | SDK |
| Run 19 (GH-399) | 16 (all) | 27 | 15,973 | — | CLI |

Run 19 is the first complete from-scratch generation, covering all releases including
infrastructure (rel00.0) and previously-problematic utilities (du, wc, ls, ts).

## References

- `generation-gh-399-generate-code-start` — snapshot before code generation
- `generation-gh-399-generate-code-finished` — snapshot after last stitch
- `generation-gh-399-generate-code-merged` — merged into feature branch
- `generation-gh-399-generate-code-with-tests` — final code state including echo/ts tests
- https://github.com/petar-djukic/go-unix-utils/issues/399 — GH-399: recurring generation run
- https://github.com/petar-djukic/go-unix-utils/pull/471 — PR #471 (generation code)
- https://github.com/petar-djukic/go-unix-utils/pull/474 — PR #474 (echo/ts tests)
