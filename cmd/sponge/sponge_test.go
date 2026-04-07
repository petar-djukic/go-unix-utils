// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	// R1.3 (passthrough): stdout comparison via RunDiffTests.
	passthroughTests := []testutils.DiffTest{
		{Name: "passthrough_small", Stdin: []byte("hello world\n")},
		{Name: "passthrough_empty", Stdin: []byte{}},
		{Name: "passthrough_multiline", Stdin: []byte("line1\nline2\nline3\n")},
	}
	testutils.RunDiffTests(t, goBin, refBin, passthroughTests)

	// R1.1/R1.2: file output tests via custom comparison.
	fileTests := []struct {
		name  string
		stdin []byte
	}{
		{"file_small", []byte("hello\n")},
		{"file_multiline", []byte("line1\nline2\nline3\n")},
		{"file_empty", []byte{}},
	}
	for _, tc := range fileTests {
		t.Run(tc.name, func(t *testing.T) {
			compareFileOutput(t, goBin, refBin, tc.stdin)
		})
	}

	// R1.1: soak-before-write contract verification.
	t.Run("soak_before_write", func(t *testing.T) {
		testSoakBeforeWrite(t, goBin)
	})
}

// compareFileOutput runs both binaries with file output and compares results.
// R6.1: compares content of output file, not stdout.
func compareFileOutput(t *testing.T, goBin, refBin string, stdin []byte) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	refOut := filepath.Join(refDir, "out.txt")
	goOut := filepath.Join(goDir, "out.txt")

	runSponge(t, refBin, refOut, stdin)
	runSponge(t, goBin, goOut, stdin)

	refContent, err := os.ReadFile(refOut)
	if err != nil {
		t.Fatalf("reading ref output: %v", err)
	}
	goContent, err := os.ReadFile(goOut)
	if err != nil {
		t.Fatalf("reading go output: %v", err)
	}
	if !bytes.Equal(refContent, goContent) {
		t.Errorf("file content mismatch\nexpected: %q\nactual:   %q",
			refContent, goContent)
	}
}

// runSponge executes a sponge binary with the given output file and stdin.
func runSponge(t *testing.T, bin, outFile string, stdin []byte) {
	t.Helper()
	cmd := exec.Command(bin, outFile)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\noutput: %s", bin, err, out)
	}
}

// testSoakBeforeWrite verifies that sponge reads all stdin before
// opening the output file, preventing data loss when reading and
// writing the same file.
func testSoakBeforeWrite(t *testing.T, bin string) {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	content := []byte("original content\n")
	if err := os.WriteFile(f, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("cat '%s' | '%s' '%s'", f, bin, f))
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("soak test failed: %v\noutput: %s", err, out)
	}
	result, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("reading result: %v", err)
	}
	if !bytes.Equal(content, result) {
		t.Errorf("soak-before-write contract violated\n"+
			"expected: %q\nactual:   %q", content, result)
	}
}
