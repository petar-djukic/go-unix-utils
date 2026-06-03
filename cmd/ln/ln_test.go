// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd037-ln R4.1, R4.2, R4.3.
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
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not found")
	}

	t.Run("error_missing_operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "no_args",
				Args:     []string{},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("error_hard_link_directory", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "hard_link_dir",
				Args:     []string{"subdir", "link"},
				WorkDir:  dir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("error_existing_destination", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src", "aaa")
		writeFile(t, dir, "dest", "bbb")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "exists_no_force",
				Args:     []string{"src", "dest"},
				WorkDir:  dir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("error_nonexistent_target", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "missing_target",
				Args:     []string{"nonexistent", "link"},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("error_multi_target_not_dir", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "a", "a")
		writeFile(t, dir, "b", "b")
		writeFile(t, dir, "notdir", "c")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "multi_not_dir",
				Args:     []string{"a", "b", "notdir"},
				WorkDir:  dir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("hard_link_basic", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"source": "content"},
			[]string{"source", "link1"},
			func(t *testing.T, dir string) {
				verifyHardLink(t, dir, "source", "link1")
			},
		)
	})

	t.Run("hard_link_multi_dir", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"a": "aa", "b": "bb"},
			[]string{"a", "b", "dest"},
			func(t *testing.T, dir string) {
				verifyHardLink(t, dir, "a", filepath.Join("dest", "a"))
				verifyHardLink(t, dir, "b", filepath.Join("dest", "b"))
			},
		)
	})

	t.Run("symlink_basic", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"target": "data"},
			[]string{"-s", "target", "slink"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, "slink", "target")
			},
		)
	})

	t.Run("symlink_to_dir", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			nil,
			[]string{"-s", "subdir", "dirlink"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, "dirlink", "subdir")
			},
		)
	})

	t.Run("symlink_long_flag", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"target": "data"},
			[]string{"--symbolic", "target", "slink2"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, "slink2", "target")
			},
		)
	})

	t.Run("force_overwrite_hard", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"src": "new", "existing": "old"},
			[]string{"-f", "src", "existing"},
			func(t *testing.T, dir string) {
				verifyHardLink(t, dir, "src", "existing")
			},
		)
	})

	t.Run("force_overwrite_symlink", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"existing": "old"},
			[]string{"-sf", "newtarget", "existing"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, "existing", "newtarget")
			},
		)
	})

	t.Run("verbose_hard_link", func(t *testing.T) {
		runLinkTestV(t, goBin, refBin,
			map[string]string{"src": "data"},
			[]string{"-v", "src", "vlink"},
			func(t *testing.T, dir string) {
				verifyHardLink(t, dir, "src", "vlink")
			},
		)
	})

	t.Run("verbose_symlink", func(t *testing.T) {
		runLinkTestV(t, goBin, refBin,
			map[string]string{"target": "data"},
			[]string{"-sv", "target", "vslink"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, "vslink", "target")
			},
		)
	})

	t.Run("verbose_force", func(t *testing.T) {
		runLinkTestV(t, goBin, refBin,
			map[string]string{"src": "new", "dest": "old"},
			[]string{"-fv", "src", "dest"},
			func(t *testing.T, dir string) {
				verifyHardLink(t, dir, "src", "dest")
			},
		)
	})

	t.Run("no_dereference_symlink_to_dir", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			os.Mkdir(filepath.Join(dir, "realdir"), 0o755)
			os.Symlink("realdir", filepath.Join(dir, "dirlink"))
			writeFile(t, dir, "src", "data")
		}, []string{"-sfn", "src", "dirlink"}, func(t *testing.T, dir string) {
			verifySymlink(t, dir, "dirlink", "src")
		})
	})

	t.Run("relative_symlink", func(t *testing.T) {
		runLinkTestV(t, goBin, refBin,
			map[string]string{"target": "data"},
			[]string{"-srv", "target", "rlink"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, "rlink", "target")
			},
		)
	})

	t.Run("relative_symlink_subdir", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "data")
			os.Mkdir(filepath.Join(dir, "sub"), 0o755)
		}, []string{"-sr", "src", "sub/link"}, func(t *testing.T, dir string) {
			target, err := os.Readlink(filepath.Join(dir, "sub", "link"))
			if err != nil {
				t.Fatalf("readlink: %v", err)
			}
			if target != "../src" {
				t.Fatalf("expected relative target '../src', got %q", target)
			}
		})
	})

	t.Run("backup_simple", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
		}, []string{"-b", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest~", "old")
		})
	})

	t.Run("backup_simple_symlink", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "dest", "old")
		}, []string{"-sb", "newtarget", "dest"}, func(t *testing.T, dir string) {
			verifySymlink(t, dir, "dest", "newtarget")
			verifyFileContent(t, dir, "dest~", "old")
		})
	})

	t.Run("backup_numbered", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
		}, []string{"--backup=numbered", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest.~1~", "old")
		})
	})

	t.Run("backup_numbered_increments", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
			writeFile(t, dir, "dest.~1~", "older")
		}, []string{"--backup=numbered", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest.~1~", "older")
			verifyFileContent(t, dir, "dest.~2~", "old")
		})
	})

	t.Run("backup_existing_falls_back_simple", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
		}, []string{"--backup=existing", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest~", "old")
		})
	})

	t.Run("backup_existing_uses_numbered", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
			writeFile(t, dir, "dest.~1~", "v1")
		}, []string{"--backup=existing", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest.~1~", "v1")
			verifyFileContent(t, dir, "dest.~2~", "old")
		})
	})

	t.Run("backup_none", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src", "new")
		writeFile(t, dir, "dest", "old")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "backup_none",
				Args:     []string{"--backup=none", "src", "dest"},
				WorkDir:  dir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("backup_custom_suffix", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
		}, []string{"-b", "-S", ".bak", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest.bak", "old")
		})
	})

	t.Run("backup_suffix_equals", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
		}, []string{"-b", "--suffix=.orig", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest.orig", "old")
		})
	})

	t.Run("backup_verbose", func(t *testing.T) {
		runLinkTestV(t, goBin, refBin,
			map[string]string{"src": "new", "dest": "old"},
			[]string{"-bv", "src", "dest"},
			func(t *testing.T, dir string) {
				verifyHardLink(t, dir, "src", "dest")
			},
		)
	})

	t.Run("backup_verbose_symlink", func(t *testing.T) {
		runLinkTestV(t, goBin, refBin,
			map[string]string{"dest": "old"},
			[]string{"-sbv", "newtarget", "dest"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, "dest", "newtarget")
			},
		)
	})

	t.Run("backup_force_combined", func(t *testing.T) {
		runLinkTestN(t, goBin, refBin, func(t *testing.T, dir string) {
			writeFile(t, dir, "src", "new")
			writeFile(t, dir, "dest", "old")
		}, []string{"-fb", "src", "dest"}, func(t *testing.T, dir string) {
			verifyHardLink(t, dir, "src", "dest")
			verifyFileContent(t, dir, "dest~", "old")
		})
	})

	t.Run("backup_method_aliases", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			method string
		}{
			{"t", "t"},
			{"never", "never"},
			{"nil", "nil"},
			{"off", "off"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				goDir := t.TempDir()
				refDir := t.TempDir()
				writeFile(t, goDir, "src", "new")
				writeFile(t, goDir, "dest", "old")
				writeFile(t, refDir, "src", "new")
				writeFile(t, refDir, "dest", "old")
				args := []string{"--backup=" + tc.method, "src", "dest"}
				goRes := runBin(t, goBin, args, goDir)
				refRes := runBin(t, refBin, args, refDir)
				compareResults(t, args, goRes, refRes)
			})
		}
	})

	t.Run("multi_target_dir", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"x": "xx", "y": "yy", "z": "zz"},
			[]string{"x", "y", "z", "dest"},
			func(t *testing.T, dir string) {
				verifyHardLink(t, dir, "x", filepath.Join("dest", "x"))
				verifyHardLink(t, dir, "y", filepath.Join("dest", "y"))
				verifyHardLink(t, dir, "z", filepath.Join("dest", "z"))
			},
		)
	})

	t.Run("multi_target_symlink_dir", func(t *testing.T) {
		runLinkTest(t, goBin, refBin,
			map[string]string{"a": "aa", "b": "bb"},
			[]string{"-s", "a", "b", "dest"},
			func(t *testing.T, dir string) {
				verifySymlink(t, dir, filepath.Join("dest", "a"), "a")
				verifySymlink(t, dir, filepath.Join("dest", "b"), "b")
			},
		)
	})

	t.Run("help_flag", func(t *testing.T) {
		goRes := runBin(t, goBin, []string{"--help"}, t.TempDir())
		refRes := runBin(t, refBin, []string{"--help"}, t.TempDir())
		if goRes.exitCode != refRes.exitCode {
			t.Fatalf("exit code: go=%d ref=%d", goRes.exitCode, refRes.exitCode)
		}
		if len(goRes.stdout) == 0 {
			t.Fatal("--help produced no stdout")
		}
	})

	t.Run("version_flag", func(t *testing.T) {
		goRes := runBin(t, goBin, []string{"--version"}, t.TempDir())
		refRes := runBin(t, refBin, []string{"--version"}, t.TempDir())
		if goRes.exitCode != refRes.exitCode {
			t.Fatalf("exit code: go=%d ref=%d", goRes.exitCode, refRes.exitCode)
		}
		if len(goRes.stdout) == 0 {
			t.Fatal("--version produced no stdout")
		}
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupDir(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		writeFile(t, dir, name, content)
	}
}

func runLinkTest(
	t *testing.T, goBin, refBin string,
	files map[string]string,
	args []string,
	verify func(t *testing.T, dir string),
) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()

	needsDestDir := false
	if len(args) > 2 {
		last := args[len(args)-1]
		nonFlagCount := 0
		for _, a := range args {
			if !isFlag(a) {
				nonFlagCount++
			}
		}
		if nonFlagCount > 2 {
			needsDestDir = true
			_ = last
		}
	}

	for _, d := range []string{goDir, refDir} {
		setupDir(t, d, files)
		if needsDestDir {
			last := args[len(args)-1]
			os.Mkdir(filepath.Join(d, last), 0o755)
		}
	}

	goRes := runBin(t, goBin, args, goDir)
	refRes := runBin(t, refBin, args, refDir)
	compareResults(t, args, goRes, refRes)

	if verify != nil {
		t.Run("verify_go", func(t *testing.T) { verify(t, goDir) })
		t.Run("verify_ref", func(t *testing.T) { verify(t, refDir) })
	}
}

func runLinkTestV(
	t *testing.T, goBin, refBin string,
	files map[string]string,
	args []string,
	verify ...func(t *testing.T, dir string),
) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	setupDir(t, goDir, files)
	setupDir(t, refDir, files)
	goRes := runBin(t, goBin, args, goDir)
	refRes := runBin(t, refBin, args, refDir)
	compareResults(t, args, goRes, refRes)

	if len(verify) > 0 && verify[0] != nil {
		t.Run("verify_go", func(t *testing.T) { verify[0](t, goDir) })
		t.Run("verify_ref", func(t *testing.T) { verify[0](t, refDir) })
	}
}

func runLinkTestN(
	t *testing.T, goBin, refBin string,
	setup func(t *testing.T, dir string),
	args []string,
	verify func(t *testing.T, dir string),
) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	setup(t, goDir)
	setup(t, refDir)
	goRes := runBin(t, goBin, args, goDir)
	refRes := runBin(t, refBin, args, refDir)
	compareResults(t, args, goRes, refRes)

	if verify != nil {
		t.Run("verify_go", func(t *testing.T) { verify(t, goDir) })
		t.Run("verify_ref", func(t *testing.T) { verify(t, refDir) })
	}
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

func verifyHardLink(t *testing.T, dir, orig, link string) {
	t.Helper()
	origPath := filepath.Join(dir, orig)
	linkPath := filepath.Join(dir, link)
	origInfo, err := os.Stat(origPath)
	if err != nil {
		t.Fatalf("stat original %s: %v", orig, err)
	}
	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat link %s: %v", link, err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("%s is a symlink, expected hard link", link)
	}
	if !os.SameFile(origInfo, linkInfo) {
		t.Fatalf("%s and %s do not share the same inode", orig, link)
	}
}

func verifySymlink(t *testing.T, dir, link, expectedTarget string) {
	t.Helper()
	linkPath := filepath.Join(dir, link)
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat %s: %v", link, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink, expected symbolic link", link)
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink %s: %v", link, err)
	}
	if target != expectedTarget {
		t.Fatalf("symlink %s: expected target %q, got %q", link, expectedTarget, target)
	}
}

func verifyFileContent(t *testing.T, dir, name, expected string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if string(content) != expected {
		t.Fatalf("file %s: expected %q, got %q", name, expected, string(content))
	}
}

type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
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
	goStdout := normalizeBinaryName(goRes.stdout)
	refStdout := normalizeBinaryName(refRes.stdout)
	goStderr := normalizeBinaryName(goRes.stderr)
	refStderr := normalizeBinaryName(refRes.stderr)

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

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?ln\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("ln"))
}
