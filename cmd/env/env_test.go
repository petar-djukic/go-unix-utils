// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/env against GNU genv.
// Covers prd039-env R1.1 (environment print), R1.2 (command execution),
// R1.3 (exit codes 127/126), R2.1 (-i ignore), R2.2 (-u unset),
// R2.3 (NAME=VALUE setting), R3.1 (-0/--null NUL output),
// R3.2 (exit code passthrough), R3.3 (invalid option exit 125),
// R4.1-R4.3 (differential test coverage).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU genv and Go env.
// Handles binary name differences, "Try --help" lines, and capitalization.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`\S*g?env\b`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("env"))
		b = tryHelp.ReplaceAll(b, nil)
		b = bytes.ToLower(b)
		return b
	}
}

// envSortNormalizer sorts output lines to account for platform-dependent
// environment variable ordering. macOS libc's setenv prepends new entries,
// producing reverse insertion order vs Go's append-based slice building.
func envSortNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		s := string(b)
		if s == "" {
			return b
		}
		// Preserve trailing newline.
		trailing := ""
		if strings.HasSuffix(s, "\n") {
			trailing = "\n"
			s = strings.TrimSuffix(s, "\n")
		}
		lines := strings.Split(s, "\n")
		sort.Strings(lines)
		return []byte(strings.Join(lines, "\n") + trailing)
	}
}

// nullSortNormalizer sorts NUL-delimited output entries for deterministic
// comparison between Go and reference binary.
func nullSortNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		s := string(b)
		if s == "" {
			return b
		}
		// Split on NUL, sort, rejoin with NUL.
		parts := strings.Split(s, "\x00")
		// Remove trailing empty part from final NUL.
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
		sort.Strings(parts)
		return []byte(strings.Join(parts, "\x00") + "\x00")
	}
}

// TestDiff runs differential tests for env core behavior.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skipf("reference binary genv not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()
	sortNorm := envSortNormalizer()
	nullSort := nullSortNormalizer()

	tests := []testutils.DiffTest{
		// R1.1: print empty environment (deterministic via -i).
		{
			Name: "print_empty_env",
			Args: []string{"-i"},
		},
		// R1.1: print single set variable.
		{
			Name: "print_single_var",
			Args: []string{"-i", "FOO=bar"},
		},
		// R1.1: print multiple set variables (sorted to handle ordering).
		{
			Name:      "print_multi_vars",
			Args:      []string{"-i", "A=1", "B=2", "C=3"},
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R2.3: later NAME=VALUE overrides earlier one for same NAME.
		{
			Name: "override_var",
			Args: []string{"-i", "X=first", "X=second"},
		},
		// R1.2: execute a simple command.
		{
			Name: "exec_echo",
			Args: []string{"/bin/echo", "hello", "world"},
		},
		// R1.2: execute command with modified environment.
		{
			Name: "exec_with_set_var",
			Args: []string{"-i", "MY_VAR=test_value",
				"/bin/sh", "-c", "echo $MY_VAR"},
		},
		// R1.3: command not found exits 127.
		{
			Name:      "cmd_not_found",
			Args:      []string{"nonexistent_cmd_xyz_98765"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.2/R3.2: exit code passthrough from command.
		{
			Name: "exit_code_passthrough",
			Args: []string{"/bin/sh", "-c", "exit 42"},
		},
		// R2.1: -i starts with empty environment.
		{
			Name: "ignore_env_short",
			Args: []string{"-i", "ONLY=this"},
		},
		// R2.1: --ignore-environment long form.
		{
			Name: "ignore_env_long",
			Args: []string{"--ignore-environment", "ONLY=this"},
		},
		// R2.1: bare '-' is alias for -i.
		{
			Name: "dash_as_ignore",
			Args: []string{"-", "CLEAN=yes"},
		},
		// R2.2: -u removes a variable from inherited environment.
		{
			Name: "unset_via_command",
			Args: []string{"-u", "ENVTEST_UNSET_VAR",
				"/bin/sh", "-c", "echo ${ENVTEST_UNSET_VAR:-gone}"},
			Env: []string{"ENVTEST_UNSET_VAR=was_set"},
		},
		// R2.2: -uNAME combined short form.
		{
			Name: "unset_combined_flag",
			Args: []string{"-uENVTEST_COMBINED",
				"/bin/sh", "-c", "echo ${ENVTEST_COMBINED:-removed}"},
			Env: []string{"ENVTEST_COMBINED=was_here"},
		},
		// R2.2: --unset=NAME long form.
		{
			Name: "unset_long_form",
			Args: []string{"--unset=ENVTEST_LONG",
				"/bin/sh", "-c", "echo ${ENVTEST_LONG:-absent}"},
			Env: []string{"ENVTEST_LONG=present"},
		},
		// R2.2: multiple -u flags remove multiple variables.
		{
			Name: "unset_multiple",
			Args: []string{"-u", "ENVTEST_M1", "-u", "ENVTEST_M2",
				"/bin/sh", "-c",
				"echo ${ENVTEST_M1:-x} ${ENVTEST_M2:-y} ${ENVTEST_M3:-z}"},
			Env: []string{"ENVTEST_M1=a", "ENVTEST_M2=b", "ENVTEST_M3=c"},
		},
		// R2.3: NAME=VALUE with command execution.
		{
			Name: "set_var_for_command",
			Args: []string{"-i", "GREETING=hello",
				"/bin/sh", "-c", "echo $GREETING"},
		},
		// R2.1 + R2.3: -i combined with -u and NAME=VALUE.
		{
			Name: "ignore_unset_set_combo",
			Args: []string{"-i", "-u", "NONEXISTENT", "KEY=val"},
		},
		// R3.1: -0 NUL-delimited output with -i for determinism.
		{
			Name: "null_delim_short",
			Args: []string{"-i", "-0", "A=1", "B=2"},
			Normalize: []testutils.NormalizeFunc{nullSort},
		},
		// R3.1: --null long form NUL-delimited output.
		{
			Name: "null_delim_long",
			Args: []string{"-i", "--null", "X=hello"},
		},
		// R3.1: -0 with empty environment produces empty output.
		{
			Name: "null_delim_empty",
			Args: []string{"-i", "-0"},
		},
		// R3.1: -0 combined with other short flags.
		{
			Name: "null_delim_combined",
			Args: []string{"-i0", "VAR=test"},
		},
		// R3.2: exit code 0 passthrough.
		{
			Name: "exit_code_zero",
			Args: []string{"/bin/sh", "-c", "exit 0"},
		},
		// R3.2: exit code 1 passthrough.
		{
			Name: "exit_code_one",
			Args: []string{"/bin/sh", "-c", "exit 1"},
		},
		// R3.3: invalid option exits 125.
		{
			Name:      "invalid_option_long",
			Args:      []string{"--bogus-flag"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.3: invalid short option exits 125.
		{
			Name:      "invalid_option_short",
			Args:      []string{"-z"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2: NAME=VALUE with value containing equals sign.
		{
			Name: "value_with_equals",
			Args: []string{"-i", "KEY=a=b=c"},
		},
		// R4.3: command not found with -i.
		{
			Name:      "cmd_not_found_with_ignore",
			Args:      []string{"-i", "nonexistent_cmd_xyz_98765"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCannotExecute tests exit code 126 for files that exist but
// are not executable (R1.3).
func TestDiffCannotExecute(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skipf("reference binary genv not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	// Create a non-executable file.
	tmpDir := t.TempDir()
	noExecFile := filepath.Join(tmpDir, "noexec")
	if wErr := os.WriteFile(noExecFile, []byte("#!/bin/sh\necho hi\n"), 0o644); wErr != nil {
		t.Fatalf("create non-executable file: %v", wErr)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "cannot_execute",
			Args:      []string{noExecFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
