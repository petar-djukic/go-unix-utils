// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying
// Go reimplementations of Unix utilities against GNU reference binaries.
//
// The harness executes a Go binary and a Homebrew GNU reference binary with
// identical stdin, arguments, and environment, then compares stdout, stderr,
// and exit code byte-for-byte after applying optional normalizers.
//
// Implements prd001-testutils.
package testutils
