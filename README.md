# go-utils

**go-utils** is a systems engineering project focused on the high-fidelity regeneration of classic Unix utilities in Go. The project follows a strict **Spec-to-Code** methodology, reverse-engineering functionality from standard Unix tools and verifying them against reference binaries.

Developed on macOS, the suite targets functional parity with GNU and Moreutils versions available via Homebrew, ensuring that the Go implementations are robust, cross-platform, and concurrent.

---

## 🛠 Project Philosophy

Unlike simple clones, this project treats Unix utilities as a set of formal requirements. Every utility is built following a four-stage pipeline:

1.  **Extract:** Clone the C/Perl source (coreutils, moreutils, grep, findutils) and derive formal requirements from the source of truth — not man pages.
2.  **Specify:** Write a PRD (numbered requirements, acceptance criteria) and a use case (concrete end-to-end flow) for each utility. Shared libraries (`pkg/`) get their own PRDs when requirements span multiple utilities.
3.  **Synthesize:** The `mage-claude-orchestrator` library drives a Claude agent to generate Go source from the PRD and use case.
4.  **Verify:** A differential testing harness (`pkg/testutils`) runs both the Go binary and the Homebrew GNU reference binary on the same inputs and compares output, exit codes, and error streams.

---

## 📊 Utility Catalog

The full catalog of 123 target utilities — covering all of coreutils, moreutils, grep, and findutils — is in [`docs/utilities.yaml`](docs/utilities.yaml). Each entry records the source file, language, estimated LOC, shared internal packages, required Go packages, testability, and an implementation profile.

**Difficulty breakdown** (how hard to port to Go):

| Difficulty  | Count | What makes it hard |
|-------------|------:|---------------------|
| trivial     |    23 | Thin wrappers; a single syscall or straightforward text output |
| easy        |    56 | Simple flag parsing and line-oriented I/O; no concurrency |
| medium      |    27 | Non-trivial flag interactions, locale handling, or moderate algorithms |
| hard        |    10 | Complex state machines, multi-column layout, or heavy stdlib gaps |
| very_hard   |     7 | Locale collation, external sort, fork/exec orchestration, or LD_PRELOAD |

**Memory model breakdown** (how data flows through the program):

| Model           | Count | Description |
|-----------------|------:|-------------|
| streaming       |    93 | Line-at-a-time; constant memory regardless of input size |
| windowed        |    12 | Keeps a bounded buffer (e.g., `tail -n 10`, `head -c 1M`) |
| accumulating    |     9 | Reads all input into memory before writing (e.g., `sort`, `uniq -c`) |
| spill_capable   |     2 | Accumulates but spills to disk when RAM is exhausted (`sort`, `sponge`) |
| filesystem_walk |     7 | Memory proportional to directory tree depth/width (`find`, `du`, `ls -R`) |

**Key findings:**
- `stdbuf` cannot be implemented in pure Go — it requires injecting a shared library via `LD_PRELOAD` and is excluded from the roadmap.
- Only `golang.org/x/sys` is permitted as an external dependency; all other functionality is written in-house.
- 7 very-hard utilities (`sort`, `join`, `awk`, `sed`, `xargs -P`, `find`, `expr`) require significant Go-specific design work around locale collation, external merge sort, or parallel process management.

---

## 📂 Repository Structure

```text
go-utils/
├── README.md           # Project manifesto and roadmap
├── go.mod              # Go module definition
├── pkg/                # Shared internal logic (The "Unix Toolkit")
│   ├── sys/            # Darwin/Linux syscalls and signal handling
│   ├── format/         # Table alignment, colors, and unit conversion
│   └── testutils/      # Harness for differential testing
└── cmd/                # Regenerated Utilities
    ├── ts/             # Timestamping (from moreutils)
    ├── sponge/         # Atomic soak-and-write (from moreutils)
    ├── ls/             # High-complexity file listing (from coreutils)
    └── vidir/          # Directory editing (from moreutils)
```
