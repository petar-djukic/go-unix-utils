// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path strips last component.
		{Name: "simple_path", Args: []string{"/usr/bin/sort"}},
		// R1.1: nested path.
		{Name: "nested_path", Args: []string{"/a/b/c"}},
		// R1.2: no slash returns dot.
		{Name: "no_slash", Args: []string{"stdio.h"}},
		// R1.2: single filename with no directory.
		{Name: "plain_filename", Args: []string{"file.txt"}},
		// R1.3: root path returns root.
		{Name: "root", Args: []string{"/"}},
		// R1.3: multiple slashes returns root.
		{Name: "multiple_slashes", Args: []string{"///"}},
		// R1.1+R1.4: trailing slashes stripped before extraction.
		{Name: "trailing_slash", Args: []string{"/usr/bin/"}},
		// R1.4: trailing slashes on result stripped.
		{Name: "trailing_slashes_nested", Args: []string{"/usr///bin///sort"}},
		// R1.2: dot path.
		{Name: "dot", Args: []string{"."}},
		// R1.2: double-dot path.
		{Name: "double_dot", Args: []string{".."}},
		// R1.1: multiple arguments.
		{Name: "multiple_args", Args: []string{"/usr/bin", "/etc/passwd"}},
		// R1.1: relative path with directory.
		{Name: "relative_path", Args: []string{"dir/file"}},
		// R1.3+R1.4: single slash in middle.
		{Name: "root_file", Args: []string{"/file"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
