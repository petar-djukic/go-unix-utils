// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R4.1: No output file, write to stdout.
			Name:  "sponge_passthrough_stdout",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		{
			// R1.1, R2.1: Small stdin written to output file.
			Name:  "sponge_small_stdin_to_file",
			Args:  []string{"outfile.txt"},
			Stdin: generateSeq(1, 100),
		},
		{
			// R1.1, R2.1: Empty stdin creates empty output file.
			Name:  "sponge_empty_stdin",
			Args:  []string{"empty_out.txt"},
			Stdin: []byte{},
		},
		{
			// R3.1, R3.3: Append mode prepends original file content.
			Name:          "sponge_append_mode",
			Args:          []string{"-a", "existing.txt"},
			Stdin:         []byte("appended line\n"),
			ExpectedFiles: map[string][]byte{"existing.txt": []byte("original line\n")},
		},
		{
			// R3.2: Append mode with non-existent file creates new file.
			Name:  "sponge_append_mode_no_existing_file",
			Args:  []string{"-a", "newfile.txt"},
			Stdin: []byte("new content\n"),
		},
		{
			// R1.3: Large stdin (>1 MB) to verify handling of large inputs.
			Name:  "sponge_large_stdin",
			Args:  []string{"large_out.txt"},
			Stdin: generateSeq(1, 50000),
		},
		{
			// R2.3: Overwrite existing file.
			Name:          "sponge_overwrite_existing",
			Args:          []string{"data.txt"},
			Stdin:         []byte("new content\n"),
			ExpectedFiles: map[string][]byte{"data.txt": []byte("old content\n")},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// generateSeq generates lines "1\n2\n...end\n" matching seq output.
func generateSeq(start, end int) []byte {
	// Pre-allocate with estimated size.
	buf := make([]byte, 0, (end-start+1)*4)
	for i := start; i <= end; i++ {
		buf = fmt.Appendf(buf, "%d\n", i)
	}
	return buf
}
