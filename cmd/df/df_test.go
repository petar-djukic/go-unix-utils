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

func helpLineNormalizer(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("Try '")) {
			lines[i] = []byte("Try 'df --help' for more information.")
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	errNorm := []testutils.NormalizeFunc{programNameNormalizer}
	exclusiveNorm := []testutils.NormalizeFunc{programNameNormalizer, helpLineNormalizer}

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
		// R3.5: include only apfs filesystems
		{Name: "type-filter", Args: []string{"-t", "apfs", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.5: long flag form
		{Name: "type-filter-long", Args: []string{"--type=apfs", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.6: exclude devfs filesystems
		{Name: "exclude-type", Args: []string{"-x", "devfs", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.6: long flag form
		{Name: "exclude-type-long", Args: []string{"--exclude-type=devfs", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.5+R3.6: same type both selected and excluded is an error
		{
			Name:      "type-include-exclude",
			Args:      []string{"-t", "apfs", "-x", "apfs", "/"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: errNorm,
		},
		// R3.5: type filter with type display
		{Name: "type-filter-display", Args: []string{"-t", "apfs", "-T", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.7: output column selection
		{Name: "output-columns", Args: []string{"--output=source,size,used,avail,pcent,target", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.7: bare --output (all fields)
		{Name: "output-all", Args: []string{"--output", "/"}, Env: []string{"LC_ALL=C"}},
		// R3.7: --output incompatible with -i
		{
			Name:      "output-inodes-exclusive",
			Args:      []string{"--output", "-i", "/"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: exclusiveNorm,
		},
		// R3.7: --output incompatible with -T
		{
			Name:      "output-type-exclusive",
			Args:      []string{"--output", "-T", "/"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: exclusiveNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
