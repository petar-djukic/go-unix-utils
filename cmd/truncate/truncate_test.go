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

var binaryNameRe = regexp.MustCompile(`(?:/\S+/)?g?truncate`)

func normBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("truncate"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	t.Run("no-args", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no-args",
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("no-size-or-ref", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no-size-or-ref",
				Args:      []string{"file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("missing-file-operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "missing-file-operand",
				Args:      []string{"-s", "100"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("invalid-size", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "invalid-size-alpha",
				Args:      []string{"-s", "abc", "file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
			{
				Name:      "invalid-size-empty",
				Args:      []string{"-s", "", "file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
			{
				Name:      "invalid-size-plus-only",
				Args:      []string{"-s", "+", "file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("invalid-option", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "invalid-option",
				Args:      []string{"-x"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("help", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "help",
				Args:      []string{"--help"},
				Normalize: []testutils.NormalizeFunc{discardStdout},
			},
		})
	})

	t.Run("version", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "version",
				Args:      []string{"--version"},
				Normalize: []testutils.NormalizeFunc{discardStdout},
			},
		})
	})

	t.Run("absolute-size", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "100", "file"}, nil)
	})

	t.Run("size-with-unit", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "1K", "file"}, nil)
	})

	t.Run("relative-grow", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "+50", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 100), 0o644)
			})
	})

	t.Run("relative-shrink", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "-30", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 100), 0o644)
			})
	})

	t.Run("shrink-below-zero", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "-200", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 50), 0o644)
			})
	})

	t.Run("at-most", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "<5", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 10), 0o644)
			})
	})

	t.Run("at-most-already-smaller", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "<20", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 10), 0o644)
			})
	})

	t.Run("at-least", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", ">20", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 5), 0o644)
			})
	})

	t.Run("at-least-already-larger", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", ">5", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 10), 0o644)
			})
	})

	t.Run("round-down", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "/3", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 7), 0o644)
			})
	})

	t.Run("round-up", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "%3", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 7), 0o644)
			})
	})

	t.Run("division-by-zero-rounddown", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "/0", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 5), 0o644)
			})
	})

	t.Run("division-by-zero-roundup", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "%0", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 5), 0o644)
			})
	})

	t.Run("no-create", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-c", "-s", "100", "noexist"}, nil)
	})

	t.Run("no-create-long", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"--no-create", "--size=100", "noexist"}, nil)
	})

	t.Run("create-new-file", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "100", "newfile"}, nil)
	})

	t.Run("multiple-files", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "50", "f1", "f2", "f3"}, nil)
	})

	t.Run("reference-file", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-r", "ref", "target"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "ref"), make([]byte, 42), 0o644)
			})
	})

	t.Run("reference-with-relative-size", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-r", "ref", "-s", "+5", "target"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "ref"), make([]byte, 11), 0o644)
			})
	})

	t.Run("nonexistent-reference", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-r", "nosuchref", "target"}, nil)
	})

	t.Run("combined-cs", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-cs", "100", "file"}, nil)
	})

	t.Run("size-zero", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-s", "0", "file"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file"), make([]byte, 50), 0o644)
			})
	})

	t.Run("io-blocks", func(t *testing.T) {
		runTruncTest(t, goBin, refBin, []string{"-o", "-s", "1", "file"}, nil)
	})
}

type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func runTruncTest(
	t *testing.T, goBin, refBin string,
	args []string, setup func(string),
) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	if setup != nil {
		setup(goDir)
		setup(refDir)
	}
	goRes := runBin(t, goBin, args, goDir)
	refRes := runBin(t, refBin, args, refDir)
	compareResults(t, args, goRes, refRes)
	compareDirSizes(t, args, goDir, refDir)
}

func runBin(t *testing.T, binary string, args []string, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", binary)
	}
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to start binary %s: %v", binary, err)
		}
	}
	return binResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

func compareResults(t *testing.T, args []string, goRes, refRes binResult) {
	t.Helper()
	goStdout := normBinaryName(goRes.stdout)
	refStdout := normBinaryName(refRes.stdout)
	goStderr := normBinaryName(goRes.stderr)
	refStderr := normBinaryName(refRes.stderr)

	if !bytes.Equal(goStdout, refStdout) ||
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
			args, refStdout, goStdout, refStderr, goStderr, refRes.exitCode, goRes.exitCode)
	}
}

func compareDirSizes(t *testing.T, args []string, goDir, refDir string) {
	t.Helper()
	goEntries, _ := os.ReadDir(goDir)
	refEntries, _ := os.ReadDir(refDir)
	goFiles := fileInfoMap(goEntries, goDir)
	refFiles := fileInfoMap(refEntries, refDir)

	for name, refSize := range refFiles {
		goSize, ok := goFiles[name]
		if !ok {
			t.Errorf("file %q exists in ref dir but not go dir (args: %v)", name, args)
			continue
		}
		if goSize != refSize {
			t.Errorf("file %q size mismatch: go=%d ref=%d (args: %v)",
				name, goSize, refSize, args)
		}
	}
	for name := range goFiles {
		if _, ok := refFiles[name]; !ok {
			t.Errorf("file %q exists in go dir but not ref dir (args: %v)", name, args)
		}
	}
}

func fileInfoMap(entries []os.DirEntry, dir string) map[string]int64 {
	m := make(map[string]int64)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		m[e.Name()] = fi.Size()
	}
	return m
}
