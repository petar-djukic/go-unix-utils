// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides portable syscall abstractions for Darwin and Linux.
//
// Implements prd002-sys: FileInfo struct with extended metadata (R2),
// Stat/Lstat functions (R2.1), terminal width and TTY detection (R1),
// SIGPIPE handler (R1.5), and SIGWINCH callback registration (R3).
package sys
