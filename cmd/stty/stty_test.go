// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstty")
	if err != nil {
		t.Skip("reference binary not found")
	}
	discardOut := testutils.NormalizeFunc(func([]byte) []byte { return nil })
	tests := []testutils.DiffTest{
		{Name: "default-no-tty", ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "all-flag", Args: []string{"-a"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "all-long", Args: []string{"--all"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "save-flag", Args: []string{"-g"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "save-long", Args: []string{"--save"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "file-dev-null", Args: []string{"-F", "/dev/null"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "file-long", Args: []string{"--file=/dev/null"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-echo", Args: []string{"echo"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-neg-echo", Args: []string{"-echo"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-intr", Args: []string{"intr", "^C"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-eof", Args: []string{"eof", "^D"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-erase-undef", Args: []string{"erase", "undef"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-min", Args: []string{"min", "1"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "combo-sane", Args: []string{"sane"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "combo-raw", Args: []string{"raw"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "combo-cooked", Args: []string{"cooked"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "combo-evenp", Args: []string{"evenp"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "combo-oddp", Args: []string{"oddp"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "speed-9600", Args: []string{"9600"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "speed-ispeed", Args: []string{"ispeed", "9600"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "speed-ospeed", Args: []string{"ospeed", "9600"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "invalid-device", Args: []string{"-F", "/dev/nonexistent_stty_test"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "invalid-arg", Args: []string{"xyznotavalidarg"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "invalid-speed", Args: []string{"12345"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "help", Args: []string{"--help"}, ExitCode: 0, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "version", Args: []string{"--version"}, ExitCode: 0, Normalize: []testutils.NormalizeFunc{discardOut}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
