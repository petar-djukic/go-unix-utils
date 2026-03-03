// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling, providing a
// stable interface that cmd/ packages use to avoid platform-specific code in
// utility implementations. Only syscalls that have platform divergence between
// Darwin and Linux, or that Go's standard library does not expose cleanly,
// belong here.
//
// Implements prd002-sys R1, R2, R3.
package sys
