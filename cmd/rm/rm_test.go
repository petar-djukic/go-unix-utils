// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not found")
	}

	t.Run("dot_rejection", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "refuse_dot",
				Args:      []string{"."},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("dotdot_rejection", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "refuse_dotdot",
				Args:      []string{".."},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("nonexistent_file", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_such_file",
				Args:      []string{"nonexistent_file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("directory_without_r", func(t *testing.T) {
		workDir := t.TempDir()
		os.Mkdir(filepath.Join(workDir, "mydir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "is_a_directory",
				Args:      []string{"mydir"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("single_file", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"file.txt"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)
			})
	})

	t.Run("multiple_files", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"a.txt", "b.txt", "c.txt"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
				os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)
				os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0o644)
			})
	})

	t.Run("partial_failure_with_nonexistent", func(t *testing.T) {
		setup := func(dir string) {
			os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("data"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"exists.txt", "nonexistent"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("file_and_directory_without_r", func(t *testing.T) {
		setup := func(dir string) {
			os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644)
			os.Mkdir(filepath.Join(dir, "mydir"), 0o755)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"file.txt", "mydir"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("recursive_directory", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-r", "d"},
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755)
				os.WriteFile(filepath.Join(dir, "d", "sub", "f.txt"), []byte("x"), 0o644)
				os.WriteFile(filepath.Join(dir, "d", "top.txt"), []byte("y"), 0o644)
			})
	})

	t.Run("force_nonexistent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "force_nonexistent",
				Args:     []string{"-f", "no_such_file"},
				ExitCode: 0,
			},
		})
	})

	t.Run("force_recursive", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-rf", "d"},
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "d", "a", "b"), 0o755)
				os.WriteFile(filepath.Join(dir, "d", "a", "b", "c.txt"), []byte("z"), 0o644)
			})
	})

	t.Run("empty_dir_with_d", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-d", "empty"},
			func(dir string) {
				os.Mkdir(filepath.Join(dir, "empty"), 0o755)
			})
	})

	t.Run("nonempty_dir_with_d", func(t *testing.T) {
		setup := func(dir string) {
			os.Mkdir(filepath.Join(dir, "notempty"), 0o755)
			os.WriteFile(filepath.Join(dir, "notempty", "f"), []byte("x"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"-d", "notempty"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("verbose_single_file", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-v", "f.txt"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "f.txt"), []byte("data"), 0o644)
			})
	})

	t.Run("verbose_recursive", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-rv", "d"},
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755)
				os.WriteFile(filepath.Join(dir, "d", "sub", "f.txt"), []byte("x"), 0o644)
			})
	})

	t.Run("force_no_args", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "force_no_operands",
				Args:     []string{"-f"},
				ExitCode: 0,
			},
		})
	})

	t.Run("interactive_always_yes", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-i", "f.txt"},
			[]byte("y\n"),
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "f.txt"), []byte("data"), 0o644)
			})
	})

	t.Run("interactive_always_no", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-i", "f.txt"},
			[]byte("n\n"),
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "f.txt"), []byte("data"), 0o644)
			})
	})

	t.Run("interactive_always_empty_file", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-i", "empty.txt"},
			[]byte("y\n"),
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "empty.txt"), nil, 0o644)
			})
	})

	t.Run("interactive_always_recursive", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-ri", "d"},
			[]byte("y\ny\ny\ny\n"),
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755)
				os.WriteFile(filepath.Join(dir, "d", "sub", "f.txt"), nil, 0o644)
			})
	})

	t.Run("interactive_always_recursive_decline_descend", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-ri", "d"},
			[]byte("n\n"),
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755)
				os.WriteFile(filepath.Join(dir, "d", "sub", "f.txt"), nil, 0o644)
			})
	})

	t.Run("interactive_once_four_files_yes", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-I", "a", "b", "c", "d"},
			[]byte("y\n"),
			func(dir string) {
				for _, name := range []string{"a", "b", "c", "d"} {
					os.WriteFile(filepath.Join(dir, name), nil, 0o644)
				}
			})
	})

	t.Run("interactive_once_four_files_no", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-I", "a", "b", "c", "d"},
			[]byte("n\n"),
			func(dir string) {
				for _, name := range []string{"a", "b", "c", "d"} {
					os.WriteFile(filepath.Join(dir, name), nil, 0o644)
				}
			})
	})

	t.Run("interactive_once_three_files_no_prompt", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-I", "a", "b", "c"},
			nil,
			func(dir string) {
				for _, name := range []string{"a", "b", "c"} {
					os.WriteFile(filepath.Join(dir, name), nil, 0o644)
				}
			})
	})

	t.Run("interactive_once_recursive_yes", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-rI", "d"},
			[]byte("y\n"),
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755)
				os.WriteFile(filepath.Join(dir, "d", "sub", "f.txt"), nil, 0o644)
			})
	})

	t.Run("interactive_once_recursive_no", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-rI", "d"},
			[]byte("n\n"),
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "d", "sub"), 0o755)
				os.WriteFile(filepath.Join(dir, "d", "sub", "f.txt"), nil, 0o644)
			})
	})

	t.Run("interactive_never_via_flag", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "interactive_never",
				Args:      []string{"--interactive=never", "nonexistent"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("interactive_always_verbose", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-iv", "f.txt"},
			[]byte("y\n"),
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "f.txt"), []byte("data"), 0o644)
			})
	})

	t.Run("force_overrides_interactive", func(t *testing.T) {
		runInteractiveTest(t, goBin, refBin, []string{"-if", "f.txt"},
			nil,
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "f.txt"), []byte("data"), 0o644)
			})
	})

	t.Run("permission_denied", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("test requires non-root user")
		}
		setup := func(dir string) {
			restricted := filepath.Join(dir, "restricted")
			os.Mkdir(restricted, 0o755)
			os.WriteFile(filepath.Join(restricted, "f.txt"), []byte("x"), 0o644)
			os.Chmod(restricted, 0o555)
		}
		cleanup := func(dir string) {
			os.Chmod(filepath.Join(dir, "restricted"), 0o755)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		t.Cleanup(func() { cleanup(goDir); cleanup(refDir) })
		args := []string{"restricted/f.txt"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})
}

func runInteractiveTest(t *testing.T, goBin, refBin string, args []string, stdin []byte, setup func(string)) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	setup(goDir)
	setup(refDir)
	goRes := runBinStdin(t, goBin, args, goDir, stdin)
	refRes := runBinStdin(t, refBin, args, refDir, stdin)
	compareResults(t, args, goRes, refRes)
}

func runRemovalTest(t *testing.T, goBin, refBin string, args []string, setup func(string)) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	setup(goDir)
	setup(refDir)
	goRes := runBin(t, goBin, args, goDir)
	refRes := runBin(t, refBin, args, refDir)
	compareResults(t, args, goRes, refRes)
}

type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func runBin(t *testing.T, binary string, args []string, workDir string) binResult {
	t.Helper()
	return runBinStdin(t, binary, args, workDir, nil)
}

func runBinStdin(t *testing.T, binary string, args []string, workDir string, stdin []byte) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", binary)
	}
	exitCode := 0
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to start binary %s: %v", binary, err)
		}
	}
	return binResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

func compareResults(t *testing.T, args []string, goRes, refRes binResult) {
	t.Helper()
	goStderr := normalizeBinaryName(goRes.stderr)
	refStderr := normalizeBinaryName(refRes.stderr)
	if !bytes.Equal(goRes.stdout, refRes.stdout) ||
		!bytes.Equal(goStderr, refStderr) ||
		goRes.exitCode != refRes.exitCode {
		t.Fatalf("divergence detected\n"+
			"  args:       %v\n"+
			"  ref stdout: %q\n"+
			"  go  stdout: %q\n"+
			"  ref stderr: %q\n"+
			"  go  stderr: %q\n"+
			"  ref exit:   %d\n"+
			"  go  exit:   %d\n",
			args, refRes.stdout, goRes.stdout, refStderr, goStderr,
			refRes.exitCode, goRes.exitCode)
	}
}

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?rm\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("rm"))
}
