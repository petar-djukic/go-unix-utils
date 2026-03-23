// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling.
//
// Implements prd002-sys: FileInfo struct for extended file metadata,
// Stat and Lstat for portable file information, TerminalWidth and
// IsTerminal for terminal queries, OnTerminalResize for SIGWINCH
// handling, and InstallSIGPIPEHandler for broken-pipe exit behavior.
package sys
