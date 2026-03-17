// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import "runtime"

// processorType returns the processor type for -p, matching GNU uname behavior.
// On Darwin, GNU uname outputs the configure-time host_cpu value: "arm" on ARM
// Macs, "x86_64" on Intel Macs. We derive this from runtime.GOARCH.
func processorType() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm"
	case "amd64":
		return "x86_64"
	default:
		return "unknown"
	}
}
