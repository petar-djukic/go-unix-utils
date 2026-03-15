// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R3.5-R3.6: TempDirNormalizer and ErrorPrefixNormalizer.

package testutils

import "regexp"

// tempDirPlaceholder is the fixed string that replaces temp directory paths.
// R3.5: deterministic placeholder for non-deterministic temp paths.
const tempDirPlaceholder = "<TEMPDIR>"

// tempDirPattern matches common temporary directory paths across platforms:
//   - /tmp/... (Linux, macOS symlink)
//   - /private/tmp/... (macOS real path)
//   - /var/folders/.../T/... (macOS per-user temp)
//   - /private/var/folders/.../T/... (macOS real per-user temp)
//
// R3.5: covers standard temp directory locations on Linux and macOS.
var tempDirPattern = regexp.MustCompile(
	`/(?:private/)?(?:tmp|var/folders)/\S+`,
)

// TempDirNormalizer replaces temporary directory paths with a deterministic
// placeholder so that differential tests pass despite path differences between
// runs. Matches /tmp/, /private/tmp/, /var/folders/, and /private/var/folders/
// path prefixes followed by non-whitespace characters.
//
// R3.5: Built-in normalizer for temp directory path patterns.
var TempDirNormalizer NormalizeFunc = func(b []byte) []byte {
	return tempDirPattern.ReplaceAll(b, []byte(tempDirPlaceholder))
}

// errorPrefixPlaceholder is the fixed string that replaces program name prefixes
// in error messages.
// R3.6: deterministic placeholder for binary-name-dependent error prefixes.
const errorPrefixPlaceholder = "<PROG>: "

// errorPrefixPattern matches the program name prefix at the start of error message
// lines. Unix utilities emit errors in the format "program: message". The GNU
// reference binaries installed via Homebrew use a "g" prefix (e.g., "gmktemp"),
// while Go binaries use the base name (e.g., "mktemp"). This pattern matches
// a lowercase program-name-like token followed by ": " at the start of a line.
//
// R3.6: matches program name followed by ": " at line start.
var errorPrefixPattern = regexp.MustCompile(`(?m)^g?[a-z][a-z0-9_-]*: `)

// ErrorPrefixNormalizer replaces program name prefixes in error messages with a
// deterministic placeholder so that differential tests pass despite the Go binary
// and reference binary having different names (e.g., "mktemp: error" vs
// "gmktemp: error" both become "<PROG>: error").
//
// R3.6: Built-in normalizer for binary-name error prefixes.
var ErrorPrefixNormalizer NormalizeFunc = func(b []byte) []byte {
	return errorPrefixPattern.ReplaceAll(b, []byte(errorPrefixPlaceholder))
}
