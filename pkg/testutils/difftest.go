// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import "regexp"

// NormalizeFunc is a type alias for functions that transform raw output bytes
// before comparison. Using a type alias (=) allows bare func literals to be
// assigned without conversion.
//
// R1.4: Named type alias in package testutils.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. Each test runs both the Go
// binary under test and the GNU reference binary with identical Args, Stdin, and
// Env, then compares their outputs.
//
// R1.1: DiffTest struct with all required fields.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string

	// Args are command-line arguments passed to both binaries.
	Args []string

	// Stdin is fed to both binaries on stdin. nil means both binaries receive
	// EOF immediately. An empty non-nil slice ([]byte{}) also produces no bytes
	// but is semantically distinct (open stdin that is immediately closed).
	// R1.2.
	Stdin []byte

	// Env contains KEY=VALUE pairs merged into the inherited environment.
	// nil means use defaults only (LC_ALL=C); non-nil pairs override or extend
	// the inherited environment. R1.3.
	Env []string

	// WorkDir sets the working directory for both binaries. Empty means the
	// harness creates a per-test t.TempDir().
	WorkDir string

	// ExitCode is the expected exit code for both binaries.
	ExitCode int

	// Normalize contains functions applied in order to stdout and stderr of
	// both binaries before comparison. nil or empty means no normalization.
	Normalize []NormalizeFunc

	// ExpectedFiles maps relative paths to expected byte content after
	// execution, for file-output utilities (sponge, cp).
	ExpectedFiles map[string][]byte
}

// timestampPlaceholder is the fixed string that replaces timestamp patterns.
const timestampPlaceholder = "<TIMESTAMP>"

// timestampPatterns matches common timestamp formats found in utility output:
//   - ISO-8601:  2026-03-14 12:34:56 or 2026-03-14T12:34:56
//   - ctime/ls:  Mar 14 12:34:56 or Mar 14  2026
//   - Unix epoch with decimals: 1710412496.123456
//   - HH:MM:SS with optional fractional seconds
var timestampPatterns = regexp.MustCompile(
	`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?` + // ISO-8601
		`|[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` + // ctime: Mar 14 12:34:56
		`|[A-Z][a-z]{2}\s+\d{1,2}\s+\d{4}` + // ls year: Mar 14  2026
		`|\d{10,}\.\d+` + // Unix epoch with decimals
		`|\d{2}:\d{2}:\d{2}(?:\.\d+)?`, // HH:MM:SS with optional fractional
)

// TimestampNormalizer replaces common timestamp patterns with a deterministic
// placeholder so that differential tests pass despite wall-clock differences.
//
// R4.2: Built-in normalizer for strftime timestamp patterns.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return timestampPatterns.ReplaceAll(b, []byte(timestampPlaceholder))
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in left-to-right order: the output of fns[i] becomes the input of
// fns[i+1]. Returns nil when no functions are provided.
//
// R1.3, R4.4: Convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	if len(fns) == 0 {
		return nil
	}
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}
