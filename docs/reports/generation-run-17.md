# Generation Run 17: 7,194 LOC, $38.93 — Full spec_complete sweep (rel02.1–rel05.4)

## Configuration

| Field | Value |
|---|---|
| Generation branches | `generation-gh-336-generate-code` (tee), `r02.2`, `r05.0`, `r05.1`, `r05.2`, `r05.3`, `r05.4` |
| Feature branch | `gh-336-generate-code-from-specs` |
| Base branch | `main` |
| Cobbler version | v0.20260306.5 |
| Model | claude-sonnet-4-6 |
| Mode | cli |
| Releases targeted | `02.1`, `02.2`, `05.0`, `05.1`, `05.2`, `05.3`, `05.4` (all spec_complete, sequentially) |
| Cycles configured | 6 |
| Total measures | 38 |
| Total stitches | 30 |
| max_stitch_issues_per_cycle | 1 |
| max_measure_issues | 1 |

## Summary

We ran generation run 17 to sweep all remaining spec_complete releases (rel02.1 through
rel05.4, excluding the deferred rel99.2/ts). The run confirmed two cobbler-scaffold bug
fixes: GH-847 (skip implemented releases) and GH-849 (Loc-Prod-After in CLI mode). Both
fixes worked correctly throughout the run. Seven generation branches produced 17 commands
across 7 releases in a single session, generating 7,194 total LOC at $38.93.

## Pre-run Conditions

- cobbler-scaffold upgraded from v0.20260306.3 to v0.20260306.5 (GH-328) in the same
  session. Required `GOPROXY=direct` to bypass proxy lag: v0.20260306.5 was not yet on
  the module proxy when the upgrade ran.
- GH-847 fix (skip implemented releases): confirmed at the first generator:run. Log showed
  `selectNextPendingUseCase: next pending UC=rel02.1-uc001-tee status=spec_complete`,
  correctly bypassing rel02.0 (already `status: implemented`).
- GH-849 fix (Loc-Prod-After always 0 in CLI mode): confirmed across multiple stitches;
  stitch commit messages correctly annotate produced LOC.

## Per-Release Results

Table: Results by release

| Generation | Release | Commands | Prod LOC | Test LOC | Total LOC | Measures | Stitches | Cost USD |
|---|---|---|---|---|---|---|---|---|
| generate-code | rel02.1 — tee | 1 | 267 | 267 | 534 | 4 | 2 | $2.35 |
| r02.2 | rel02.2 — head, seq | 2 | 648 | 353 | 1,001 | 3 | 2 | $4.41 |
| r05.0 | rel05.0 — echo, tac, nl, fold | 4 | 1,154 | 706 | 1,860 | 5 | 4 | $8.26 |
| r05.1 | rel05.1 — expand, unexpand | 2 | 625 | 287 | 912 | 3 | 2 | $3.30 |
| r05.2 | rel05.2 — cut, paste | 2 | 779 | 241 | 1,020 | 3 | 2 | $2.77 |
| r05.3 | rel05.3 — uniq, comm | 2 | 522 | 181 | 703 | 3 | 2 | $3.75 |
| r05.4 | rel05.4 — md5sum, sha1sum, sha256sum, sha512sum | 4 | 960 | 204 | 1,164 | 17 | 16 | $14.09 |
| **Total** | **7 releases** | **17** | **4,955** | **2,239** | **7,194** | **38** | **30** | **$38.93** |

## Cost Analysis

Table: Cost summary

| Category | Runs | Cost USD |
|---|---|---|
| rel02.1 (tee) | 4 measures + 2 stitches | $2.35 |
| rel02.2 (head, seq) | 3 measures + 2 stitches | $4.41 |
| rel05.0 (echo, tac, nl, fold) | 5 measures + 4 stitches | $8.26 |
| rel05.1 (expand, unexpand) | 3 measures + 2 stitches | $3.30 |
| rel05.2 (cut, paste) | 3 measures + 2 stitches | $2.77 |
| rel05.3 (uniq, comm) | 3 measures + 2 stitches | $3.75 |
| rel05.4 (checksums) | 17 measures + 16 stitches | $14.09 |
| **Total** | | **$38.93** |

Total LOC produced: 4,955 production + 2,239 test = 7,194 LOC
Cost per KLOC: $38.93 / 7.194 KLOC = **$5.41/KLOC**

## Generated Packages

Table: Packages produced (by release)

| Package | Prod LOC | Test LOC | Release |
|---|---|---|---|
| `cmd/tee` | ~145 | 267 | rel02.1 |
| `cmd/head` | ~330 | 178 | rel02.2 |
| `cmd/seq` | ~270 | 175 | rel02.2 |
| `cmd/echo` | ~150 | ~150 | rel05.0 |
| `cmd/tac` | ~200 | ~150 | rel05.0 |
| `cmd/nl` | ~500 | ~200 | rel05.0 |
| `cmd/fold` | ~300 | ~200 | rel05.0 |
| `cmd/expand` | ~300 | ~140 | rel05.1 |
| `cmd/unexpand` | ~250 | ~145 | rel05.1 |
| `cmd/cut` | ~450 | 0 | rel05.2 (no tests — see GH-377) |
| `cmd/paste` | ~310 | 241 | rel05.2 |
| `cmd/uniq` | ~340 | 0 | rel05.3 (no tests — see GH-377) |
| `cmd/comm` | ~250 | 181 | rel05.3 |
| `cmd/md5sum` | ~212 | 51 | rel05.4 |
| `cmd/sha1sum` | ~212 | 51 | rel05.4 |
| `cmd/sha256sum` | ~212 | 51 | rel05.4 |
| `cmd/sha512sum` | ~212 | 51 | rel05.4 |

Note: pkg/sys/sigpipe.go and pkg/testutils/testutils.go were added or extended during
the run; their LOC is included in the production totals above.

## Failures and Restarts

Table: Issues encountered during run

| Type | Description | Resolution |
|---|---|---|
| Credential expiry | Claude session expired at generator:start for rel02.1 | User ran `/login`, then `generator:resume` |
| Leftover branch | `generation-gh-262-generate-code-from-specs` from run 15 blocked generator:start | Force-deleted with `git branch -D` |
| Test artifacts committed | Tee stitch agent wrote files named `1` and `2` (content: `x`) to worktree root during test execution; both were committed | Deleted, staged removal, committed |
| rel05.4 cycle limit | Checksum quartet required 17+ measure+stitch cycles, exceeding `cycles: 6` | Required 3 `generator:resume` calls; completed on 3rd resume |

## Cobbler-scaffold Issues Filed

None new. GH-847 and GH-849 were both confirmed fixed by v0.20260306.5:

- GH-847: implemented release skipping works; measure correctly targets rel02.1 after rel02.0
- GH-849: Loc-Prod-After is now correct in stitch commit annotations in CLI mode

## Follow-up Issues Filed

| Issue | Summary |
|---|---|
| GH-377 | Add missing differential tests for cmd/cut and cmd/uniq |

## Observations

rel05.4 (checksum quartet) was unexpectedly expensive: 17 measures, 16 stitches, $14.09
(36% of the total run cost) for 4 structurally identical utilities. The measure agent
decomposed the task into many small follow-up tasks rather than one comprehensive stitch.
This is a measure prompt inefficiency; a single stitch produced all 4 checksum commands
(main.go + test file, same structure across all four) in one call once the right task was
proposed. The per-utility repetition in the measure phase drove up the cycle count.

All 7 releases completed their differential tests (passing or skipping when reference
binaries are absent). The `go build ./...` verification passed cleanly across all
generated code before each `generator:stop`. No stitch failures in the non-checksum
releases.

Table: Prior run comparison

| Run | Issue | Date | Prod LOC | Test LOC | Total LOC | Cost USD | Cost/KLOC | Notes |
|---|---|---|---|---|---|---|---|---|
| 14 | GH-206 | 2026-03-04 | 2,238 | 3,319 | 5,557 | $59.67 | $10.74 | 8 commands, podman |
| 15 | GH-262 | 2026-03 | — | — | 3,904 | $45.85 | $11.74 | 12 tasks, headers mode |
| 16 | GH-313 | 2026-03-06 | 625 | 464 | 1,089 | $4.60 | $4.22 | CLI mode trial, trivial utilities |
| **17** | **GH-336** | **2026-03-06** | **4,955** | **2,239** | **7,194** | **$38.93** | **$5.41** | Full spec_complete sweep, 17 commands |

Run 17 cost/KLOC of $5.41 is consistent with run 16 ($4.22) and reflects the CLI mode
efficiency over runs 14-15 which used podman. The higher total cost is proportional to
the larger scope (17 commands vs 5 in run 16).
