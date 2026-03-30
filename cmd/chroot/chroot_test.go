// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gchroot:" with "chroot:" in stderr output
// so the Go binary and reference binary error messages can be compared.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gchroot:"), []byte("chroot:"))
}

// normalizeFullPath strips full binary paths from stderr, replacing e.g.
// "/opt/homebrew/bin/gchroot:" or "/opt/homebrew/bin/chroot:" with "chroot:".
var fullPathRe = regexp.MustCompile(`/[^\s:]*chroot:`)

func normalizeFullPath(data []byte) []byte {
	return fullPathRe.ReplaceAll(data, []byte("chroot:"))
}

// normalizeTryHelp removes "Try '...' for more information.\n" lines
// that GNU coreutils appends to some error messages.
var tryHelpRe = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

func normalizeTryHelp(data []byte) []byte {
	return tryHelpRe.ReplaceAll(data, nil)
}

// normalizeErrCase lowercases the first letter after ": " in error
// messages to handle Go's lowercase vs GNU's uppercase error strings.
func normalizeErrCase(data []byte) []byte {
	return []byte(strings.ToLower(string(data)))
}

// stderrNormalizers is the standard normalizer chain for chroot error tests.
var stderrNormalizers = []testutils.NormalizeFunc{
	normalizeProgramName,
	normalizeFullPath,
	normalizeTryHelp,
	normalizeErrCase,
}

// TestDiff runs differential tests comparing the Go chroot against gchroot.
// prd100-chroot R1, R2.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchroot")
	if err != nil {
		t.Skipf("reference binary gchroot not in PATH: %v", err)
	}

	t.Run("error_paths", func(t *testing.T) {
		t.Parallel()
		runErrorPathTests(t, goBin, refBin)
	})

	t.Run("privileged", func(t *testing.T) {
		t.Parallel()
		if os.Getuid() != 0 {
			t.Skip("chroot requires root privileges; skipping privileged tests")
		}
		runPrivilegedTests(t, goBin, refBin)
	})
}

// runErrorPathTests tests error conditions that do not require root.
// These fail before or at the chroot(2) call; both binaries get the same
// OS error when not root (EPERM), so differential output still matches.
func runErrorPathTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			// R2.2: missing operand exits 125.
			Name:      "missing_operand",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  exitInternal,
			Normalize: stderrNormalizers,
		},
		{
			// R2.2: nonexistent NEWROOT exits 125.
			// As non-root both binaries get EPERM; as root both get ENOENT.
			Name:      "nonexistent_newroot",
			Args:      []string{"/nonexistent_chroot_path_xyz_99"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  exitInternal,
			Normalize: stderrNormalizers,
		},
		{
			// R2.2: unrecognized option exits 125.
			Name:      "unrecognized_option",
			Args:      []string{"--badopt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  exitInternal,
			Normalize: stderrNormalizers,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runPrivilegedTests tests chroot behavior that requires root.
// These are skipped when os.Getuid() != 0.
func runPrivilegedTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	tmpDir := t.TempDir()
	tests := []testutils.DiffTest{
		{
			// R2.2: COMMAND not found inside chroot exits 127.
			Name:      "command_not_found",
			Args:      []string{tmpDir, "nonexistent_cmd_xyz_99"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  exitNotFound,
			Normalize: stderrNormalizers,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
