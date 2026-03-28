// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd105-stty R3.1–R3.3: differential tests for stty.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeStderrToEmpty clears stderr entirely for tests where the error
// message text diverges due to different OS-level wording between our Go
// implementation and the GNU C implementation. Exit code parity is the
// meaningful signal for these cases.
func normalizeStderrToEmpty(b []byte) []byte {
	// This normalizer is applied to both binaries, so both get empty stderr.
	return nil
}

// TestDiff runs differential tests comparing Go stty against gstty.
// R3.1: display modes, R3.2: settings modification, R3.3: error handling.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstty")
	if err != nil {
		t.Skipf("reference binary gstty not in PATH: %v", err)
	}

	// stty requires a controlling terminal. In test mode, stdin is a pipe,
	// so display and modification commands fail on both binaries. We verify
	// that exit codes and stdout match; stderr is normalized away because
	// error message text differs between Go and GNU implementations.
	stderrNorm := []testutils.NormalizeFunc{normalizeStderrToEmpty}

	tests := []testutils.DiffTest{
		// R3.1: display modes — both binaries error without a terminal.
		{
			Name:      "no_args_no_tty",
			Args:      []string{},
			Normalize: stderrNorm,
		},
		{
			Name:      "all_flag_no_tty",
			Args:      []string{"-a"},
			Normalize: stderrNorm,
		},
		{
			Name:      "save_flag_no_tty",
			Args:      []string{"-g"},
			Normalize: stderrNorm,
		},
		{
			Name:      "long_all_flag_no_tty",
			Args:      []string{"--all"},
			Normalize: stderrNorm,
		},
		{
			Name:      "long_save_flag_no_tty",
			Args:      []string{"--save"},
			Normalize: stderrNorm,
		},

		// R3.3: error handling — invalid settings and bad device paths.
		{
			Name:      "invalid_setting_name",
			Args:      []string{"nosuchsetting"},
			Normalize: stderrNorm,
		},
		{
			Name:      "invalid_device_path",
			Args:      []string{"-F", "/dev/nonexistent_tty_device"},
			Normalize: stderrNorm,
		},
		{
			Name:      "invalid_device_long_flag",
			Args:      []string{"--file=/dev/nonexistent_tty_device"},
			Normalize: stderrNorm,
		},
		{
			Name:      "missing_F_argument",
			Args:      []string{"-F"},
			Normalize: stderrNorm,
		},
		{
			Name:      "invalid_char_value",
			Args:      []string{"intr", "badvalue"},
			Normalize: stderrNorm,
		},
		{
			Name:      "invalid_speed",
			Args:      []string{"ispeed", "999999"},
			Normalize: stderrNorm,
		},

		// R3.2: settings modification — without a terminal these
		// also produce errors, verifying exit code parity.
		{
			Name:      "set_echo_no_tty",
			Args:      []string{"echo"},
			Normalize: stderrNorm,
		},
		{
			Name:      "disable_echo_no_tty",
			Args:      []string{"-echo"},
			Normalize: stderrNorm,
		},
		{
			Name:      "set_sane_no_tty",
			Args:      []string{"sane"},
			Normalize: stderrNorm,
		},
		{
			Name:      "set_raw_no_tty",
			Args:      []string{"raw"},
			Normalize: stderrNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
