// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd057-mv R1.1-R1.4, R2.1-R2.4, R3.1-R3.3, R4.1-R4.4.
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not found")
	}

	binaryNameRe := regexp.MustCompile(`(?m)^(?:/\S+/)?g?mv:`)
	normBin := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("mv:"))
	})
	normCase := testutils.NormalizeFunc(func(b []byte) []byte {
		return bytes.ToLower(b)
	})
	normTry := testutils.NormalizeFunc(func(b []byte) []byte {
		var out [][]byte
		for line := range bytes.SplitSeq(b, []byte("\n")) {
			if !bytes.HasPrefix(bytes.TrimSpace(bytes.ToLower(line)), []byte("try ")) {
				out = append(out, bytes.Clone(line))
			}
		}
		return bytes.Join(out, []byte("\n"))
	})
	errNorm := []testutils.NormalizeFunc{normBin, normCase, normTry}

	t.Run("r1_1_rename_file", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "hello\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "hello\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r1_1_overwrite_existing", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "new\n")
			writeFile(t, dir, "dest.txt", "old\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "new\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r1_2_multi_file_into_dir", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "a.txt", "aaa\n")
			writeFile(t, dir, "b.txt", "bbb\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"a.txt", "b.txt", "dest"}, nil)
		checkFile(t, goDir, "dest/a.txt", "aaa\n")
		checkFile(t, goDir, "dest/b.txt", "bbb\n")
		checkAbsent(t, goDir, "a.txt")
		checkAbsent(t, goDir, "b.txt")
	})

	t.Run("r1_3_move_directory", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			os.Mkdir(filepath.Join(dir, "srcdir"), 0o755)
			writeFile(t, dir, "srcdir/file.txt", "inside\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"srcdir", "destdir"}, nil)
		checkFile(t, goDir, "destdir/file.txt", "inside\n")
		checkAbsent(t, goDir, "srcdir")
	})

	t.Run("r1_4_dest_is_directory", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "file.txt", "content\n")
			os.Mkdir(filepath.Join(dir, "destdir"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"file.txt", "destdir"}, nil)
		checkFile(t, goDir, "destdir/file.txt", "content\n")
		checkAbsent(t, goDir, "file.txt")
	})

	t.Run("r1_1_source_not_found", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {}
		runMvTest(t, goBin, refBin, setup,
			[]string{"nonexistent.txt", "dest.txt"}, errNorm)
	})

	t.Run("r1_2_target_not_directory", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "a.txt", "a\n")
			writeFile(t, dir, "b.txt", "b\n")
			writeFile(t, dir, "notadir", "x\n")
		}
		runMvTest(t, goBin, refBin, setup,
			[]string{"a.txt", "b.txt", "notadir"}, errNorm)
	})

	t.Run("r2_1_interactive_yes", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "new\n")
			writeFile(t, dir, "dest.txt", "old\n")
		}
		goDir := runMvTestInput(t, goBin, refBin, setup,
			[]string{"-i", "src.txt", "dest.txt"}, errNorm, "y\n")
		checkFile(t, goDir, "dest.txt", "new\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r2_1_interactive_no", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "new\n")
			writeFile(t, dir, "dest.txt", "old\n")
		}
		goDir := runMvTestInput(t, goBin, refBin, setup,
			[]string{"-i", "src.txt", "dest.txt"}, errNorm, "n\n")
		checkFile(t, goDir, "dest.txt", "old\n")
		checkFile(t, goDir, "src.txt", "new\n")
	})

	t.Run("r2_2_force_overwrite", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "new\n")
			writeFile(t, dir, "dest.txt", "old\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-f", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "new\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r2_2_force_after_interactive", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "new\n")
			writeFile(t, dir, "dest.txt", "old\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-i", "-f", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "new\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r2_3_no_clobber", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "new\n")
			writeFile(t, dir, "dest.txt", "old\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-n", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "old\n")
		checkFile(t, goDir, "src.txt", "new\n")
	})

	t.Run("r2_3_no_clobber_no_dest", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "hello\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-n", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "hello\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r2_4_permission_denied", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "hello\n")
			restricted := filepath.Join(dir, "nowrite")
			os.Mkdir(restricted, 0o755)
			os.Chmod(restricted, 0o555)
			t.Cleanup(func() { os.Chmod(restricted, 0o755) })
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"src.txt", "nowrite/dest.txt"}, errNorm)
		checkFile(t, goDir, "src.txt", "hello\n")
	})

	t.Run("r3_1_verbose_rename", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "hello\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-v", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "hello\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r3_1_verbose_into_dir", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "file.txt", "content\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-v", "file.txt", "dest"}, nil)
		checkFile(t, goDir, "dest/file.txt", "content\n")
		checkAbsent(t, goDir, "file.txt")
	})

	t.Run("r3_1_verbose_overwrite", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "new\n")
			writeFile(t, dir, "dest.txt", "old\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-v", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "new\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r3_1_verbose_long_flag", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "hello\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"--verbose", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "hello\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r3_1_verbose_multi_file", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "a.txt", "aaa\n")
			writeFile(t, dir, "b.txt", "bbb\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-v", "a.txt", "b.txt", "dest"}, nil)
		checkFile(t, goDir, "dest/a.txt", "aaa\n")
		checkFile(t, goDir, "dest/b.txt", "bbb\n")
	})

	t.Run("r3_2_target_directory_short", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "a.txt", "aaa\n")
			writeFile(t, dir, "b.txt", "bbb\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-t", "dest", "a.txt", "b.txt"}, nil)
		checkFile(t, goDir, "dest/a.txt", "aaa\n")
		checkFile(t, goDir, "dest/b.txt", "bbb\n")
		checkAbsent(t, goDir, "a.txt")
		checkAbsent(t, goDir, "b.txt")
	})

	t.Run("r3_2_target_directory_long", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "file.txt", "content\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"--target-directory=dest", "file.txt"}, nil)
		checkFile(t, goDir, "dest/file.txt", "content\n")
		checkAbsent(t, goDir, "file.txt")
	})

	t.Run("r3_2_target_directory_single", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "file.txt", "hello\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-t", "dest", "file.txt"}, nil)
		checkFile(t, goDir, "dest/file.txt", "hello\n")
		checkAbsent(t, goDir, "file.txt")
	})

	t.Run("r3_2_target_directory_verbose", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "file.txt", "content\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-vt", "dest", "file.txt"}, nil)
		checkFile(t, goDir, "dest/file.txt", "content\n")
		checkAbsent(t, goDir, "file.txt")
	})

	t.Run("r3_3_no_target_dir_rename", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "hello\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-T", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "hello\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r3_3_no_target_dir_replaces_empty_dir", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			os.Mkdir(filepath.Join(dir, "srcdir"), 0o755)
			writeFile(t, dir, "srcdir/file.txt", "inside\n")
			os.Mkdir(filepath.Join(dir, "destdir"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"-T", "srcdir", "destdir"}, errNorm)
		checkFile(t, goDir, "destdir/file.txt", "inside\n")
		checkAbsent(t, goDir, "srcdir")
	})

	t.Run("r3_3_no_target_dir_file_to_dir_error", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "file.txt", "content\n")
			os.Mkdir(filepath.Join(dir, "destdir"), 0o755)
		}
		runMvTest(t, goBin, refBin, setup,
			[]string{"-T", "file.txt", "destdir"}, errNorm)
	})

	t.Run("r3_3_no_target_dir_long_flag", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "src.txt", "hello\n")
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"--no-target-directory", "src.txt", "dest.txt"}, nil)
		checkFile(t, goDir, "dest.txt", "hello\n")
		checkAbsent(t, goDir, "src.txt")
	})

	t.Run("r4_3_partial_failure_continues", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "good.txt", "ok\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"missing.txt", "good.txt", "dest"}, errNorm)
		checkFile(t, goDir, "dest/good.txt", "ok\n")
		checkAbsent(t, goDir, "good.txt")
	})

	t.Run("r4_3_partial_failure_multi_errors", func(t *testing.T) {
		setup := func(t *testing.T, dir string) {
			writeFile(t, dir, "real.txt", "data\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0o755)
		}
		goDir := runMvTest(t, goBin, refBin, setup,
			[]string{"gone1.txt", "real.txt", "gone2.txt", "dest"}, errNorm)
		checkFile(t, goDir, "dest/real.txt", "data\n")
		checkAbsent(t, goDir, "real.txt")
	})
}

func runMvTest(
	t *testing.T, goBin, refBin string,
	setup func(t *testing.T, dir string),
	args []string,
	norm []testutils.NormalizeFunc,
) string {
	t.Helper()
	return runMvTestInput(t, goBin, refBin, setup, args, norm, "")
}

func runMvTestInput(
	t *testing.T, goBin, refBin string,
	setup func(t *testing.T, dir string),
	args []string,
	norm []testutils.NormalizeFunc,
	stdin string,
) string {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	setup(t, goDir)
	setup(t, refDir)
	env := buildTestEnv()
	goRes := runCmd(t, goBin, args, env, goDir, stdin)
	refRes := runCmd(t, refBin, args, env, refDir, stdin)
	compareResults(t, goRes, refRes, norm)
	return goDir
}

func runCmd(
	t *testing.T, bin string, args, env []string, dir, stdin string,
) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	if err == nil {
		return cmdResult{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return cmdResult{
			stdout:   stdout.Bytes(),
			stderr:   stderr.Bytes(),
			exitCode: exitErr.ExitCode(),
		}
	}
	t.Fatalf("failed to run %s: %v", bin, err)
	return cmdResult{}
}

func buildTestEnv() []string {
	envMap := make(map[string]string)
	for _, entry := range os.Environ() {
		key, val, _ := strings.Cut(entry, "=")
		envMap[key] = val
	}
	envMap["LC_ALL"] = "C"
	out := make([]string, 0, len(envMap))
	for k, v := range envMap {
		out = append(out, k+"="+v)
	}
	return out
}

func compareResults(
	t *testing.T, goRes, refRes cmdResult, norm []testutils.NormalizeFunc,
) {
	t.Helper()
	goOut, goErr := goRes.stdout, goRes.stderr
	refOut, refErr := refRes.stdout, refRes.stderr
	for _, fn := range norm {
		goOut = fn(goOut)
		refOut = fn(refOut)
		goErr = fn(goErr)
		refErr = fn(refErr)
	}
	if !bytes.Equal(goOut, refOut) {
		t.Errorf("stdout differs\n  go:  %q\n  ref: %q", goOut, refOut)
	}
	if !bytes.Equal(goErr, refErr) {
		t.Errorf("stderr differs\n  go:  %q\n  ref: %q", goErr, refErr)
	}
	if goRes.exitCode != refRes.exitCode {
		t.Errorf("exit code differs: go=%d ref=%d",
			goRes.exitCode, refRes.exitCode)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func checkFile(t *testing.T, dir, name, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("expected file %s: %v", name, err)
	}
	if string(got) != want {
		t.Errorf("file %s: got %q, want %q", name, got, want)
	}
}

func checkAbsent(t *testing.T, dir, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
		t.Errorf("expected %s to not exist", name)
	}
}
