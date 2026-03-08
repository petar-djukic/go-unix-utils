# go-unix-utils

Go reimplementations of 123 Unix utilities (coreutils, moreutils, grep, findutils), generated from formal specifications and verified against GNU reference binaries through differential testing.

## Architectural Thesis

Man pages are ambiguous. Unit tests with hardcoded expectations encode a human's interpretation of behavior, not the behavior itself. We take a different approach: derive specifications directly from the C source code, then verify the Go implementation by running it side-by-side with the GNU binary and comparing outputs byte-for-byte. The specification and the verification are the product; the Go binaries are a byproduct.

This project is a stress test for spec-driven AI code generation. The human writes the spec. An AI orchestrator generates the code. A differential testing harness decides whether the code is correct.

## Pipeline

```mermaid
graph LR
    A["C/Perl Source<br>(coreutils, moreutils,<br>grep, findutils)"] -->|Extract + Specify| B["PRD + Use Case<br>(formal spec)"]
    B -->|Synthesize| C["Go Source<br>(cmd/utility/)"]
    C -->|Verify| D["Differential<br>Test Harness"]
    D -->|Fail: refine spec| B
```

## Scope and Status

Specifications cover 8 releases across 11 use cases. Seven generation runs have produced code at v1 tags; the working branch carries specifications only. The most recent successful generation (run 7, eng14) produced 892 production LOC and 1,054 test LOC for foundational infrastructure packages in a single measure+stitch cycle.

| Metric | Count |
| ------ | ----: |
| Target utilities | 123 |
| PRDs written (utility) | 7 (wc, cat, sponge, du, ls core, ls extended, ts) |
| PRDs written (shared components) | 4 (pkg/testutils, pkg/sys, pkg/format, magefiles) |
| Use cases | 11 |
| Test suites | 8 (one per release) |
| Releases | 8 (5 active, 3 deferred) |
| Generation runs documented | 7 (eng04 through eng14) |
| v0 spec tags | 16 |
| v1 generation tags | 25 |

17 releases implemented: rel00.0 (infrastructure), rel01.1 (cat), rel01.2 (sponge), rel01.3 (du), rel02.0 (true, false, yes, basename, dirname), rel02.1 (tee), rel02.2 (head, seq), rel03.0 (wc), rel04.0 (ls), rel05.0 (echo, tac, nl, fold), rel05.1 (expand, unexpand), rel05.2 (cut, paste), rel05.3 (uniq, comm), rel05.4 (md5sum, sha1sum, sha256sum, sha512sum), rel05.5 (ts), rel06.0 (mkdir, rmdir, mktemp, ln, unlink), rel06.1 (env, printenv, id, whoami, groups). See [road-map.yaml](docs/road-map.yaml) for status.

## Methodology

Extract. We clone the C or Perl source repository (GNU coreutils, moreutils, grep, findutils) and read the source -- not the man page -- to catalog every flag, exit code, signal handler, and edge case. The source is the specification authority.

Specify. We write a product requirements document (PRD) with numbered requirements, acceptance criteria, and design decisions for each utility. We write a use case describing a concrete end-to-end execution path. We write a test suite with explicit inputs and expected structural outputs. Shared components (syscall abstractions, output formatting, the test harness itself) get their own PRDs when their requirements span multiple utilities.

Synthesize. The cobbler pipeline (mage-claude-orchestrator) reads the PRD and generates Go source. The human does not write Go; the orchestrator does not write specs. This separation isolates specification quality from generation quality, making each independently measurable.

Verify. A differential testing harness executes both the Go binary and the Homebrew GNU reference binary (e.g., `gwc`, `gdu`, `gcat`) with identical arguments, environment, and stdin. It compares stdout, stderr, and exit code. Normalization hooks handle acceptable differences (e.g., mtime fields in `ls -l` output). If the outputs diverge, the harness reports the triggering input, both raw outputs, and both exit codes.

## Repository Structure

```text
go-unix-utils/
├── docs/
│   ├── VISION.yaml                  Project goals, risks, boundaries
│   ├── ARCHITECTURE.yaml            Components, design decisions, tech choices
│   ├── SPECIFICATIONS.yaml          PRD, use case, and test suite index
│   ├── utilities.yaml               Full 123-utility catalog
│   ├── road-map.yaml                Release schedule and use case status
│   ├── specs/
│   │   ├── product-requirements/    Per-utility and shared component PRDs
│   │   ├── use-cases/               Concrete end-to-end execution paths
│   │   └── test-suites/             Explicit inputs and expected outputs
│   └── engineering/                 Engineering guidelines and generation run reports
├── magefiles/                       Build targets (build, lint, analyze, cobbler)
└── configuration.yaml               Orchestrator configuration

At v1 tags only:
├── cmd/                             One package per utility
└── pkg/                             Shared libraries (sys, format, testutils)
```

## Technology Choices

Go for the implementations. Go's constrained type system and lack of implicit behavior make it predictable input for AI code generation. A Claude agent producing Go introduces fewer subtle bugs than one producing C++ or Rust, because there are fewer ways to express the same logic.

Differential testing against Homebrew GNU binaries rather than unit tests with expected constants. Both binaries stat the same files on the same filesystem, so block counts, inode numbers, and file sizes are deterministic. The test is "does the Go binary produce the same output as the GNU binary?" not "does the Go binary produce the output I think is correct?"

`golang.org/x/sys` as the only external dependency. Platform-divergent syscall field names (`st_mtimespec` on Darwin vs. `st_mtim` on Linux), ioctl constants, and signal mask operations require it. Everything else is standard library. `golang.org/x/text/collate` is the planned addition when locale-sensitive utilities are scheduled.

## Build and Test

```bash
mage build          # Compile all utilities to bin/
go test ./...       # Run all tests (including differential tests)
mage lint           # Lint
mage analyze        # Cross-artifact consistency checks (PRDs, use cases, test suites, roadmap)
```

Requires Go 1.21+, [Mage](https://magefile.org/), and Homebrew GNU coreutils/moreutils (`brew install coreutils moreutils`).

## Documentation

- [VISION.yaml](docs/VISION.yaml) -- Project goals, success criteria, risks, and boundaries
- [ARCHITECTURE.yaml](docs/ARCHITECTURE.yaml) -- Component descriptions, design decisions, technology choices
- [SPECIFICATIONS.yaml](docs/SPECIFICATIONS.yaml) -- PRD, use case, and test suite index with traceability
- [Engineering guidelines](docs/engineering/) -- Conventions, generation run reports, and practices above the code

## Tagging Scheme

The repository uses a two-prefix versioning convention to distinguish human-written specifications from AI-generated code.

`v0.YYYYMMDD.N` tags mark specification-only releases. The repository at a v0 tag contains PRDs, use cases, test suites, architecture documents, and build infrastructure -- everything a human wrote. No Go implementation code is present. These tags represent the input to the generation pipeline.

`v1.YYYYMMDD.N` tags mark generation releases. The repository at a v1 tag contains everything from the corresponding v0 baseline plus the Go source code that the cobbler pipeline produced. Each v1 tag is the output of a complete generation run: one or more measure+stitch cycles that read the specifications and wrote the implementations.

The intended lifecycle: a human writes specifications and tags v0, the pipeline generates code and tags v1, then the generated code is removed from the working branch so that the next specification edit starts from a clean v0 state. This separation makes it possible to re-generate all code from any v0 tag, compare v1 tags across generation runs, and measure specification quality independently of generation quality.

Seven generation runs are documented in [engineering reports](docs/engineering/) (eng04 through eng14). Generated code lives at v1 tags; the working branch carries specifications only.
