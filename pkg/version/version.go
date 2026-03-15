// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package version provides the build version string for all cmd/ binaries.
// Implements prd059-version R1.5: exported variable importable by other cmd/ packages.
//
// The Version variable is set at build time via:
//
//	-ldflags "-X github.com/petar-djukic/go-unix-utils/pkg/version.Version=<tag>"
//
// When not set, it defaults to "dev" for development builds (R1.2).
package version

// Version is the build version string. It defaults to "dev" and is overridden
// at build time via ldflags. Other cmd/ packages import this variable to report
// the same version without duplicating the ldflags mechanism.
var Version = "dev"
