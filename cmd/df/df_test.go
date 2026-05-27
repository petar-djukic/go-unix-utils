// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func programNameNormalizer(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gdf:"), []byte("df:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	errNorm := []testutils.NormalizeFunc{programNameNormalizer}

	tests := []testutils.DiffTest{
		// R1.1, R1.2, R1.3: FILE argument reports containing filesystem
		{Name: "root-fs", Args: []string{"/"}, Env: []string{"LC_ALL=C"}},
		// R1.4: regular file resolves to containing filesystem
		{Name: "file-arg", Args: []string{"/etc/hosts"}, Env: []string{"LC_ALL=C"}},
		// R1.4, R1.5: multiple FILE arguments, column alignment across rows
		{Name: "multiple-files", Args: []string{"/", "/tmp"}, Env: []string{"LC_ALL=C"}},
		// R1.4: non-existent file produces diagnostic and exit 1
		{
			Name:      "nonexistent",
			Args:      []string{"/nonexistent-path-12345"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: errNorm,
		},
		// R1.4: mix of valid and invalid FILE arguments
		{
			Name:      "mixed-valid-invalid",
			Args:      []string{"/", "/nonexistent-path-12345"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: errNorm,
		},
		// R1.4: duplicate FILE arguments show duplicate rows
		{Name: "duplicate-file", Args: []string{"/", "/"}, Env: []string{"LC_ALL=C"}},
		// R1.5: column width alignment with human-readable sizes
		{Name: "h-alignment", Args: []string{"-h", "/", "/tmp"}, Env: []string{"LC_ALL=C"}},
		// R2.1: human-readable binary unit sizes
		{Name: "human-readable", Args: []string{"-h", "/"}, Env: []string{"LC_ALL=C"}},
		// R2.1: long flag form
		{Name: "human-readable-long", Args: []string{"--human-readable", "/"}, Env: []string{"LC_ALL=C"}},
		// R2.2: SI unit sizes
		{Name: "si-units", Args: []string{"-H", "/"}, Env: []string{"LC_ALL=C"}},
		// R2.2: long flag form
		{Name: "si-units-long", Args: []string{"--si", "/"}, Env: []string{"LC_ALL=C"}},
		// R2.3: last flag wins — -h after -H
		{Name: "last-h-wins", Args: []string{"-H", "-h", "/"}, Env: []string{"LC_ALL=C"}},
		// R2.3: last flag wins — -H after -h
		{Name: "last-H-wins", Args: []string{"-h", "-H", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.1: filesystem type display
		{Name: "type-display", Args: []string{"-T", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.1: long flag form
		{Name: "type-display-long", Args: []string{"--print-type", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.1: type with human-readable sizes
		{Name: "type-human", Args: []string{"-T", "-h", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.2: inode display
		{Name: "inodes", Args: []string{"-i", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.2: long flag form
		{Name: "inodes-long", Args: []string{"--inodes", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.2+R3.1: inode display with type column
		{Name: "inodes-type", Args: []string{"-i", "-T", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.3: include pseudo-filesystems with file argument
		{Name: "all-with-file", Args: []string{"-a", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.4: local filesystems only
		{Name: "local-only", Args: []string{"-l", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.4: long flag form
		{Name: "local-only-long", Args: []string{"--local", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.4: local with type display
		{Name: "local-type", Args: []string{"-l", "-T", "/"}, Env: []string{"LC_ALL=C"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
