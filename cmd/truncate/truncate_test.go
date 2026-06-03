// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?truncate\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("truncate"))
}

func normalizeErrors(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("No such file or directory"), []byte("no such file or directory"))
	b = bytes.ReplaceAll(b, []byte("Invalid number"), []byte("invalid number"))
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary not found")
	}

	errorNorm := []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrors}

	tests := []testutils.DiffTest{
		// R1.1, R1.4, R3.1: absolute size, creates missing file
		{
			Name: "set size 100",
			Args: []string{"-s", "100", "file"},
			ExpectedFiles: map[string][]byte{
				"file": make([]byte, 100),
			},
		},
		{
			Name: "set size zero",
			Args: []string{"-s", "0", "file"},
			ExpectedFiles: map[string][]byte{
				"file": {},
			},
		},
		{
			Name: "long flag size equals",
			Args: []string{"--size=200", "file"},
			ExpectedFiles: map[string][]byte{
				"file": make([]byte, 200),
			},
		},
		// R1.1: unit suffixes
		{
			Name: "K suffix",
			Args: []string{"-s", "1K", "file"},
			ExpectedFiles: map[string][]byte{
				"file": make([]byte, 1024),
			},
		},
		{
			Name: "KB suffix",
			Args: []string{"-s", "1KB", "file"},
			ExpectedFiles: map[string][]byte{
				"file": make([]byte, 1000),
			},
		},
		// R1.3: multiple files
		{
			Name: "multiple files",
			Args: []string{"-s", "50", "fileA", "fileB", "fileC"},
			ExpectedFiles: map[string][]byte{
				"fileA": make([]byte, 50),
				"fileB": make([]byte, 50),
				"fileC": make([]byte, 50),
			},
		},
		// R1.4: --no-create
		{
			Name: "no create short flag",
			Args: []string{"-c", "-s", "100", "nonexistent"},
		},
		{
			Name: "no create long flag",
			Args: []string{"--no-create", "-s", "100", "nonexistent"},
		},
		// R2.2, R3.2: missing reference file
		{
			Name:      "missing reference file",
			Args:      []string{"-r", "nosuchref", "file"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		// R3.2: error cases
		{
			Name:      "missing file operand",
			Args:      []string{"-s", "100"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		{
			Name:      "no size or reference",
			Args:      []string{"file"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		{
			Name:      "invalid size",
			Args:      []string{"-s", "abc", "file"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		{
			Name:      "empty size",
			Args:      []string{"-s", "", "file"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
	}

	// R1.1: shrink existing file preserves leading bytes
	shrinkExistDir := t.TempDir()
	os.WriteFile(filepath.Join(shrinkExistDir, "file"), bytes.Repeat([]byte("x"), 100), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "shrink existing file",
		Args:    []string{"-s", "50", "file"},
		WorkDir: shrinkExistDir,
		ExpectedFiles: map[string][]byte{
			"file": bytes.Repeat([]byte("x"), 50),
		},
	})

	// R1.2: relative grow (+)
	growDir := t.TempDir()
	os.WriteFile(filepath.Join(growDir, "file"), make([]byte, 100), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "grow by 50",
		Args:    []string{"-s", "+50", "file"},
		WorkDir: growDir,
	})

	// R1.2: relative shrink (-)
	shrinkDir := t.TempDir()
	os.WriteFile(filepath.Join(shrinkDir, "file"), make([]byte, 100), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "shrink by 20",
		Args:    []string{"-s", "-20", "file"},
		WorkDir: shrinkDir,
	})

	// R1.2: at most (<) — file larger than limit
	atMostDir := t.TempDir()
	os.WriteFile(filepath.Join(atMostDir, "file"), make([]byte, 200), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "at most 100",
		Args:    []string{"-s", "<100", "file"},
		WorkDir: atMostDir,
		ExpectedFiles: map[string][]byte{
			"file": make([]byte, 100),
		},
	})

	// R1.2: at least (>) — file smaller than limit
	atLeastDir := t.TempDir()
	os.WriteFile(filepath.Join(atLeastDir, "file"), make([]byte, 50), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "at least 100",
		Args:    []string{"-s", ">100", "file"},
		WorkDir: atLeastDir,
		ExpectedFiles: map[string][]byte{
			"file": make([]byte, 100),
		},
	})

	// R1.2: round down (/)
	roundDownDir := t.TempDir()
	os.WriteFile(filepath.Join(roundDownDir, "file"), make([]byte, 350), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "round down to multiple",
		Args:    []string{"-s", "/100", "file"},
		WorkDir: roundDownDir,
		ExpectedFiles: map[string][]byte{
			"file": make([]byte, 300),
		},
	})

	// R1.2: round up (%)
	roundUpDir := t.TempDir()
	os.WriteFile(filepath.Join(roundUpDir, "file"), make([]byte, 350), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "round up to multiple",
		Args:    []string{"-s", "%100", "file"},
		WorkDir: roundUpDir,
		ExpectedFiles: map[string][]byte{
			"file": make([]byte, 400),
		},
	})

	// R2.1: reference file short flag
	refDir1 := t.TempDir()
	os.WriteFile(filepath.Join(refDir1, "ref"), make([]byte, 75), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "reference file short flag",
		Args:    []string{"-r", "ref", "file"},
		WorkDir: refDir1,
		ExpectedFiles: map[string][]byte{
			"file": make([]byte, 75),
		},
	})

	// R2.1: reference file long flag
	refDir2 := t.TempDir()
	os.WriteFile(filepath.Join(refDir2, "ref"), make([]byte, 75), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "reference file long flag",
		Args:    []string{"--reference=ref", "file"},
		WorkDir: refDir2,
		ExpectedFiles: map[string][]byte{
			"file": make([]byte, 75),
		},
	})

	// R2.1: reference combined with relative size
	refPlusDir := t.TempDir()
	os.WriteFile(filepath.Join(refPlusDir, "ref"), make([]byte, 100), 0644)
	tests = append(tests, testutils.DiffTest{
		Name:    "reference with relative size",
		Args:    []string{"-r", "ref", "-s", "+25", "file"},
		WorkDir: refPlusDir,
		ExpectedFiles: map[string][]byte{
			"file": make([]byte, 125),
		},
	})

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestNoCreateSkipsCreation(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "absent")

	cmd := exec.Command(bin, "-c", "-s", "100", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("truncate -c failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to not exist, got: %v", err)
	}
}
