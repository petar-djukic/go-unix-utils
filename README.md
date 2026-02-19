# go-utils

**go-utils** is a systems engineering project focused on the high-fidelity regeneration of classic Unix utilities in Go. The project follows a strict **Spec-to-Code** methodology, reverse-engineering functionality from standard Unix tools and verifying them against reference binaries.

Developed on macOS, the suite targets functional parity with GNU and Moreutils versions available via Homebrew, ensuring that the Go implementations are robust, cross-platform, and concurrent.

---

## 🛠 Project Philosophy

Unlike simple clones, this project treats Unix utilities as a set of formal requirements. Every utility is built following a four-stage pipeline:

1.  **Specification:** Extracting behavior, flags, and edge cases into a `spec.md`.
2.  **Synthesis:** Implementing the utility in Go, targeting the `cmd/` directory.
3.  **Library Maturation:** Promoting repeated logic (I/O, formatting, OS syscalls) into a shared `pkg/` library.
4.  **Differential Testing:** Comparing the Go binary's output, exit codes, and error handling against GNU/Homebrew references.

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
