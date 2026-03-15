# Go Unix Utils

Go reimplementations of GNU coreutils and moreutils, verified via differential testing against Homebrew reference binaries.

## Repository Structure

```
cmd/<name>/          One directory per utility. Contains main.go and <name>_test.go.
pkg/testutils/       Differential testing framework (DiffTest, RunDiffTests, BuildBinary).
pkg/sys/             System calls: Stat, Lstat, terminal width, SIGPIPE handler.
pkg/format/          Output formatting: HumanSize, color, column alignment.
docs/specs/          PRDs, use cases, test suites (YAML). Source of truth for requirements.
docs/constitutions/  Style guides and schemas for code and documentation.
magefiles/           Build automation (mage). Do not modify unless working on prd011.
configuration.yaml   Orchestrator config. Do not modify during stitch tasks.
```

The `cmd/mktemp/collision.*` files are hash-collision test fixtures, not source code.

## Building and Testing

Build and test within the Go module. Never create Go source files outside this repository.

```bash
# Build a specific command
go build -o bin/<name> ./cmd/<name>/

# Build all packages
go build ./pkg/...

# Run tests for a specific package
go test ./pkg/<name>/... -count=1

# Run tests for a specific command
go test ./cmd/<name>/... -count=1

# Verify compilation without producing a binary
go vet ./pkg/... ./cmd/...
```

The `bin/` directory is git-ignored. Always use `-o bin/<name>` when building cmd/ binaries to avoid leaving binaries in the working directory.

Do not use `mage build` or `mage lint` for individual packages — they operate on the whole project and may fail when the repository is partially generated.

## Differential Testing Pattern

Every cmd/ package uses `pkg/testutils.RunDiffTests` to compare output against the GNU reference binary (installed via Homebrew with g-prefix: `gcat`, `gls`, `gsort`, etc.).

```go
func TestDiff(t *testing.T) {
    goBin := testutils.BuildBinary(t, ".")
    refBin, err := exec.LookPath("g<name>")
    if err != nil {
        t.Skip("reference binary not found")
    }
    tests := []testutils.DiffTest{ /* ... */ }
    testutils.RunDiffTests(t, goBin, refBin, tests)
}
```

Always call `exec.LookPath` and `t.Skip` if the reference binary is not found. Tests must pass on machines without Homebrew GNU binaries installed.

`BuildBinary(t, ".")` compiles the cmd/ package from within the module. It does not work with paths outside the module. Do not create temporary Go programs in os.TempDir() — they cannot be built because they are outside the module boundary.

## During Code Generation

When this repository is in a generation run, cmd/ directories may be empty (source code is deleted and regenerated). If a test depends on a built cmd/ binary and no Go files exist in that cmd/ directory, the test must skip gracefully:

```go
if _, err := os.Stat(filepath.Join("cmd", name, "main.go")); os.IsNotExist(err) {
    t.Skipf("cmd/%s not yet generated", name)
}
```

Plan test prerequisites before writing code. If a precondition is absent, write a skip from the start rather than writing a test that fails and then fixing it.

## Git

Git is managed externally by the cobbler orchestrator during generation runs. Do not run git commands (add, commit, status, init, rm .git) during stitch tasks.

## SIGPIPE

All cmd/ utilities that write to stdout must call `sys.InstallSIGPIPEHandler()` at the start of main to match GNU coreutils behavior when piped to consumers that close stdin early.
