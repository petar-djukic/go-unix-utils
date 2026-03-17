// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

// processorType returns the processor type for -p, matching GNU uname behavior.
// On Linux, GNU uname typically returns "unknown" for -p.
func processorType() string {
	return "unknown"
}
