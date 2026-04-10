// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rm against grm (GNU coreutils).
// Implements srd058 differential testing for R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "grm"

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// writeFile creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// mkDir creates a subdirectory in dir.
func mkDir(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

// programNameRe matches any binary path ending in rm or grm.
var programNameRe = regexp.MustCompile(`(?:\S+/)?g?rm:`)

// tryHelpRe matches "Try 'BINARY --help'" with any binary path.
var tryHelpRe = regexp.MustCompile(`Try '[^']+' for`)

// normalizeRm normalizes program name in stderr output.
func normalizeRm(data []byte) []byte {
	data = programNameRe.ReplaceAll(data, []byte("rm:"))
	data = tryHelpRe.ReplaceAll(data, []byte("Try 'rm --help' for"))
	return data
}

// runBin executes a binary and captures its output.
func runBin(
	t *testing.T, bin string, args []string,
	dir string, stdin []byte,
) binResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	err := cmd.Run()
	code := extractExitCode(t, err, bin)
	return binResult{
		stdout: stdout.Bytes(),
		stderr: stderr.Bytes(),
		exitCode: code,
	}
}

// extractExitCode gets the exit code from a command result.
func extractExitCode(t *testing.T, err error, bin string) int {
	t.Helper()
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	t.Fatalf("failed to execute %s: %v", bin, err)
	return -1
}

// runRmDiff runs a differential test with separate temp dirs.
// Creates identical directory state for ref and go binaries,
// then compares stdout, stderr, and exit code.
func runRmDiff(
	t *testing.T, goBin, refBin string,
	setup func(t *testing.T, dir string),
	args []string,
) {
	t.Helper()
	runRmDiffStdin(t, goBin, refBin, setup, args, nil)
}

// runRmDiffStdin runs a differential test with stdin input.
func runRmDiffStdin(
	t *testing.T, goBin, refBin string,
	setup func(t *testing.T, dir string),
	args []string, stdin []byte,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if setup != nil {
		setup(t, refDir)
		setup(t, goDir)
	}
	ref := runBin(t, refBin, args, refDir, stdin)
	got := runBin(t, goBin, args, goDir, stdin)
	compareRmResults(t, args, ref, got)
}

// compareRmResults compares stdout, stderr, and exit code.
func compareRmResults(
	t *testing.T, args []string,
	ref, got binResult,
) {
	t.Helper()
	refOut := normalizeRm(ref.stdout)
	gotOut := normalizeRm(got.stdout)
	if !bytes.Equal(refOut, gotOut) {
		t.Errorf("stdout mismatch\nargs: %v\nref:  %q\ngot:  %q",
			args, refOut, gotOut)
	}
	refErr := normalizeRm(ref.stderr)
	gotErr := normalizeRm(got.stderr)
	if !bytes.Equal(refErr, gotErr) {
		t.Errorf("stderr mismatch\nargs: %v\nref:  %q\ngot:  %q",
			args, refErr, gotErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch\nargs: %v\nref: %d\ngot: %d",
			args, ref.exitCode, got.exitCode)
	}
}

// TestDiff runs differential tests comparing cmd/rm against grm.
// D2: uses exec.LookPath("grm") and t.Skip if not found.
// D4: uses testutils.BuildBinary(t, ".") to compile the Go binary.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v",
			refBinName, err)
	}
	t.Run("R1.1", func(t *testing.T) { testR1_1(t, goBin, refBin) })
	t.Run("R1.2", func(t *testing.T) { testR1_2(t, goBin, refBin) })
	t.Run("R1.3", func(t *testing.T) { testR1_3(t, goBin, refBin) })
	t.Run("R1.4", func(t *testing.T) { testR1_4(t, goBin, refBin) })
	t.Run("R2.1", func(t *testing.T) { testR2_1(t, goBin, refBin) })
	t.Run("R2.2", func(t *testing.T) { testR2_2(t, goBin, refBin) })
	t.Run("R2.3", func(t *testing.T) { testR2_3(t, goBin, refBin) })
	t.Run("R2.4", func(t *testing.T) { testR2_4(t, goBin, refBin) })
	t.Run("R3.1", func(t *testing.T) { testR3_1(t, goBin, refBin) })
	t.Run("R3.2", func(t *testing.T) { testR3_2(t, goBin, refBin) })
	t.Run("R3.3", func(t *testing.T) { testR3_3(t, goBin, refBin) })
	t.Run("R3.4", func(t *testing.T) { testR3_4(t, goBin, refBin) })
	t.Run("R4.1", func(t *testing.T) { testR4_1(t, goBin, refBin) })
	t.Run("R4.2", func(t *testing.T) { testR4_2(t, goBin, refBin) })
	t.Run("R4.3", func(t *testing.T) { testR4_3(t, goBin, refBin) })
	t.Run("R4.4", func(t *testing.T) { testR4_4(t, goBin, refBin) })
}

// testR1_1 tests basic file removal.
// R1.1: must remove each FILE argument using unlink(2).
func testR1_1(t *testing.T, goBin, refBin string) {
	t.Run("single_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "hello\n")
			},
			[]string{"f.txt"},
		)
	})
	t.Run("multiple_files", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a.txt", "alpha\n")
				writeFile(t, dir, "b.txt", "beta\n")
				writeFile(t, dir, "c.txt", "gamma\n")
			},
			[]string{"a.txt", "b.txt", "c.txt"},
		)
	})
	t.Run("nonexistent_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"nonexistent.txt"},
		)
	})
}

// testR1_2 tests directory rejection without -r.
// R1.2: without -r, must refuse to remove a directory.
func testR1_2(t *testing.T, goBin, refBin string) {
	t.Run("directory_without_r", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
			},
			[]string{"subdir"},
		)
	})
	t.Run("directory_with_contents", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
				writeFile(t, dir, "subdir/file.txt", "data\n")
			},
			[]string{"subdir"},
		)
	})
}

// testR1_3 tests dot and dot-dot rejection.
// R1.3: must not remove '.' or '..'.
func testR1_3(t *testing.T, goBin, refBin string) {
	t.Run("refuse_dot", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
			},
			[]string{"-r", "subdir/."},
		)
	})
	t.Run("refuse_dotdot", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
			},
			[]string{"-r", "subdir/.."},
		)
	})
}

// testR1_4 tests error handling and continuation.
// R1.4: must print error and continue with remaining files.
func testR1_4(t *testing.T, goBin, refBin string) {
	t.Run("continue_after_error", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "good1.txt", "ok1\n")
				writeFile(t, dir, "good2.txt", "ok2\n")
			},
			[]string{"good1.txt", "missing.txt", "good2.txt"},
		)
	})
	t.Run("mixed_file_and_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "file.txt", "data\n")
				mkDir(t, dir, "subdir")
			},
			[]string{"file.txt", "subdir"},
		)
	})
	t.Run("force_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f", "nonexistent.txt"},
		)
	})
	t.Run("no_args", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{},
		)
	})
}

// testR2_1 tests recursive directory removal.
// R2.1: -r/-R/--recursive removes directories and contents.
func testR2_1(t *testing.T, goBin, refBin string) {
	t.Run("recursive_r_flag", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d/sub")
				writeFile(t, dir, "d/sub/file.txt", "data\n")
				writeFile(t, dir, "d/top.txt", "top\n")
			},
			[]string{"-r", "d"},
		)
	})
	t.Run("recursive_R_flag", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d/sub")
				writeFile(t, dir, "d/sub/file.txt", "data\n")
			},
			[]string{"-R", "d"},
		)
	})
	t.Run("recursive_long_flag", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d/sub")
				writeFile(t, dir, "d/sub/file.txt", "data\n")
			},
			[]string{"--recursive", "d"},
		)
	})
	t.Run("recursive_empty_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "emptydir")
			},
			[]string{"-r", "emptydir"},
		)
	})
	t.Run("recursive_nested_dirs", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "a/b/c")
				writeFile(t, dir, "a/b/c/deep.txt", "deep\n")
				writeFile(t, dir, "a/b/mid.txt", "mid\n")
				writeFile(t, dir, "a/top.txt", "top\n")
			},
			[]string{"-r", "a"},
		)
	})
	t.Run("recursive_verbose", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d")
				writeFile(t, dir, "d/file.txt", "data\n")
			},
			[]string{"-rv", "d"},
		)
	})
}

// testR2_2 tests force mode.
// R2.2: -f ignores non-existent files and never prompts.
func testR2_2(t *testing.T, goBin, refBin string) {
	t.Run("force_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f", "no_such_file"},
		)
	})
	t.Run("force_no_args", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f"},
		)
	})
	t.Run("force_long_flag", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"--force", "no_such_file"},
		)
	})
	t.Run("force_existing_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"-f", "f.txt"},
		)
	})
	t.Run("force_multiple_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f", "a", "b", "c"},
		)
	})
}

// testR2_3 tests combined recursive and force mode.
// R2.3: -r and -f combined silently removes directory trees.
func testR2_3(t *testing.T, goBin, refBin string) {
	t.Run("rf_directory_tree", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d/sub")
				writeFile(t, dir, "d/sub/file.txt", "data\n")
				writeFile(t, dir, "d/top.txt", "top\n")
			},
			[]string{"-rf", "d"},
		)
	})
	t.Run("rf_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-rf", "no_such_dir"},
		)
	})
	t.Run("rf_mixed_existing_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d")
				writeFile(t, dir, "d/f.txt", "data\n")
			},
			[]string{"-rf", "d", "no_such"},
		)
	})
	t.Run("rf_empty_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "emptydir")
			},
			[]string{"-rf", "emptydir"},
		)
	})
}

// testR2_4 tests empty directory removal with -d.
// R2.4: -d removes empty directories; without -d or -r, fails.
func testR2_4(t *testing.T, goBin, refBin string) {
	t.Run("d_empty_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "emptydir")
			},
			[]string{"-d", "emptydir"},
		)
	})
	t.Run("d_nonempty_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d")
				writeFile(t, dir, "d/f.txt", "data\n")
			},
			[]string{"-d", "d"},
		)
	})
	t.Run("d_long_flag", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "emptydir")
			},
			[]string{"--dir", "emptydir"},
		)
	})
	t.Run("d_verbose", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "emptydir")
			},
			[]string{"-dv", "emptydir"},
		)
	})
	t.Run("d_with_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"-d", "f.txt"},
		)
	})
}

// testR3_1 tests interactive mode prompting before every removal.
// R3.1: -i must prompt before every removal.
func testR3_1(t *testing.T, goBin, refBin string) {
	t.Run("i_yes_single_file", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"-i", "f.txt"},
			[]byte("y\n"),
		)
	})
	t.Run("i_no_single_file", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"-i", "f.txt"},
			[]byte("n\n"),
		)
	})
	t.Run("i_yes_multiple_files", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a.txt", "aa\n")
				writeFile(t, dir, "b.txt", "bb\n")
			},
			[]string{"-i", "a.txt", "b.txt"},
			[]byte("y\ny\n"),
		)
	})
	t.Run("i_recursive_dir", func(t *testing.T) {
		// descend? y, remove file? y, remove dir? y
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d")
				writeFile(t, dir, "d/f.txt", "data\n")
			},
			[]string{"-ir", "d"},
			[]byte("y\ny\ny\n"),
		)
	})
	t.Run("i_empty_file", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "empty.txt", "")
			},
			[]string{"-i", "empty.txt"},
			[]byte("y\n"),
		)
	})
}

// testR3_2 tests interactive-once mode.
// R3.2: -I prompts once before removing >3 files or recursively.
func testR3_2(t *testing.T, goBin, refBin string) {
	t.Run("I_four_files_yes", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a", "1\n")
				writeFile(t, dir, "b", "2\n")
				writeFile(t, dir, "c", "3\n")
				writeFile(t, dir, "d", "4\n")
			},
			[]string{"-I", "a", "b", "c", "d"},
			[]byte("y\n"),
		)
	})
	t.Run("I_four_files_no", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a", "1\n")
				writeFile(t, dir, "b", "2\n")
				writeFile(t, dir, "c", "3\n")
				writeFile(t, dir, "d", "4\n")
			},
			[]string{"-I", "a", "b", "c", "d"},
			[]byte("n\n"),
		)
	})
	t.Run("I_three_files_no_prompt", func(t *testing.T) {
		// Three files: no prompt needed, removes all.
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a", "1\n")
				writeFile(t, dir, "b", "2\n")
				writeFile(t, dir, "c", "3\n")
			},
			[]string{"-I", "a", "b", "c"},
		)
	})
	t.Run("I_recursive_yes", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d")
				writeFile(t, dir, "d/f.txt", "data\n")
			},
			[]string{"-Ir", "d"},
			[]byte("y\n"),
		)
	})
	t.Run("I_recursive_no", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d")
				writeFile(t, dir, "d/f.txt", "data\n")
			},
			[]string{"-Ir", "d"},
			[]byte("n\n"),
		)
	})
}

// testR3_3 tests verbose output.
// R3.3: -v must print the name of each file as it is removed.
func testR3_3(t *testing.T, goBin, refBin string) {
	t.Run("v_single_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"-v", "f.txt"},
		)
	})
	t.Run("v_multiple_files", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a.txt", "aa\n")
				writeFile(t, dir, "b.txt", "bb\n")
			},
			[]string{"-v", "a.txt", "b.txt"},
		)
	})
}

// testR3_4 tests --interactive=WHEN flag.
// R3.4: WHEN is never (like -f), once (like -I), always (like -i).
func testR3_4(t *testing.T, goBin, refBin string) {
	t.Run("interactive_always", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"--interactive=always", "f.txt"},
			[]byte("y\n"),
		)
	})
	t.Run("interactive_once_four_files", func(t *testing.T) {
		runRmDiffStdin(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a", "1\n")
				writeFile(t, dir, "b", "2\n")
				writeFile(t, dir, "c", "3\n")
				writeFile(t, dir, "d", "4\n")
			},
			[]string{"--interactive=once", "a", "b", "c", "d"},
			[]byte("y\n"),
		)
	})
	t.Run("interactive_never", func(t *testing.T) {
		// No prompt, removes file directly.
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"--interactive=never", "f.txt"},
		)
	})
	t.Run("f_overrides_i", func(t *testing.T) {
		// -i then -f: last wins, no prompt.
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"-i", "-f", "f.txt"},
		)
	})
}

// testR4_1 tests exit code 0 on successful removal.
// R4.1: must exit 0 when all files are removed successfully.
func testR4_1(t *testing.T, goBin, refBin string) {
	t.Run("exit_0_single_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "data\n")
			},
			[]string{"f.txt"},
		)
	})
	t.Run("exit_0_multiple_files", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a", "1\n")
				writeFile(t, dir, "b", "2\n")
			},
			[]string{"a", "b"},
		)
	})
	t.Run("exit_0_recursive", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "d/sub")
				writeFile(t, dir, "d/sub/f.txt", "data\n")
			},
			[]string{"-r", "d"},
		)
	})
	t.Run("exit_0_empty_dir_d", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "emptydir")
			},
			[]string{"-d", "emptydir"},
		)
	})
}

// testR4_2 tests exit code 1 on failure with continuation.
// R4.2: must exit 1 when any removal fails and continue.
func testR4_2(t *testing.T, goBin, refBin string) {
	t.Run("exit_1_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"nonexistent"},
		)
	})
	t.Run("exit_1_dir_without_r", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
			},
			[]string{"subdir"},
		)
	})
	t.Run("exit_1_continue_after_error", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "good.txt", "ok\n")
			},
			[]string{"good.txt", "missing.txt"},
		)
	})
	t.Run("exit_1_permission_denied", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				noWrite := filepath.Join(dir, "noperm")
				if err := os.Mkdir(noWrite, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				p := filepath.Join(noWrite, "protected.txt")
				if err := os.WriteFile(
					p, []byte("secret\n"), 0o644,
				); err != nil {
					t.Fatalf("write: %v", err)
				}
				// Remove write permission from parent dir.
				if err := os.Chmod(noWrite, 0o555); err != nil {
					t.Fatalf("chmod: %v", err)
				}
				t.Cleanup(func() {
					// best-effort restore for cleanup
					_ = os.Chmod(noWrite, 0o755)
				})
			},
			[]string{"noperm/protected.txt"},
		)
	})
	t.Run("exit_1_no_operand", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{},
		)
	})
}

// testR4_3 tests force mode exit codes.
// R4.3: with -f, must exit 0 even when files do not exist.
func testR4_3(t *testing.T, goBin, refBin string) {
	t.Run("f_exit_0_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f", "no_such_file"},
		)
	})
	t.Run("f_exit_0_no_args", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f"},
		)
	})
	t.Run("f_exit_0_multiple_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f", "a", "b", "c"},
		)
	})
	t.Run("rf_exit_0_nonexistent_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-rf", "no_such_dir"},
		)
	})
}

// testR4_4 tests the comprehensive differential test coverage.
// R4.4: covers single file, multi-file, -r, -f, -d, -v, errors,
// permission denied, and . / .. refusal.
func testR4_4(t *testing.T, goBin, refBin string) {
	t.Run("comprehensive_single_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "single.txt", "one\n")
			},
			[]string{"single.txt"},
		)
	})
	t.Run("comprehensive_multi_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "x", "1\n")
				writeFile(t, dir, "y", "2\n")
				writeFile(t, dir, "z", "3\n")
			},
			[]string{"x", "y", "z"},
		)
	})
	t.Run("comprehensive_recursive", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "tree/a/b")
				writeFile(t, dir, "tree/a/b/leaf.txt", "leaf\n")
				writeFile(t, dir, "tree/a/mid.txt", "mid\n")
			},
			[]string{"-r", "tree"},
		)
	})
	t.Run("comprehensive_force_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f", "ghost"},
		)
	})
	t.Run("comprehensive_d_empty_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "edir")
			},
			[]string{"-d", "edir"},
		)
	})
	t.Run("comprehensive_verbose", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "v.txt", "verbose\n")
			},
			[]string{"-v", "v.txt"},
		)
	})
	t.Run("comprehensive_error_dir_no_r", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "mydir")
			},
			[]string{"mydir"},
		)
	})
	t.Run("comprehensive_refuse_dot", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "sub")
			},
			[]string{"-rf", "sub/."},
		)
	})
	t.Run("comprehensive_refuse_dotdot", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "sub")
			},
			[]string{"-rf", "sub/.."},
		)
	})
}
