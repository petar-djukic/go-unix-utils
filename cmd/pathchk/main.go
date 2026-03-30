// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pathchk checks whether file names are valid or portable (prd103-pathchk R1, R2).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "pathchk"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses flags, iterates over pathname arguments, and dispatches checks.
// R2.1: exits 0 when all pathnames pass. R2.2: exits 1 when any fails.
func run(args []string, stderr *os.File) int {
	posix, posixExtended, paths := parseArgs(args)
	if len(paths) == 0 {
		return 0
	}
	exitCode := 0
	for _, path := range paths {
		var err error
		switch {
		case posix:
			err = checkPOSIX(path)
		case posixExtended:
			err = checkPOSIXExtended(path)
		default:
			err = checkDefault(path)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs extracts -p, -P, --portability flags and remaining pathnames.
// R1.4: supports multiple pathname arguments.
func parseArgs(args []string) (posix, posixExtended bool, paths []string) {
	for _, arg := range args {
		switch {
		case arg == "-p" || arg == "--portability":
			posix = true
		case arg == "-P":
			posixExtended = true
		case arg == "--" :
			continue
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Unknown flag; treat as pathname for now
			paths = append(paths, arg)
		default:
			paths = append(paths, arg)
		}
	}
	return posix, posixExtended, paths
}

// checkDefault checks a pathname against system limits.
// R1.1: checks component length and existence of leading directories.
// Stub: returns nil (success). Implementation fills in real logic.
func checkDefault(_ string) error {
	return nil
}

// checkPOSIX checks a pathname against POSIX portability rules.
// R1.2: only portable filename characters, component <= 14, total <= 256.
// Stub: returns nil (success). Implementation fills in real logic.
func checkPOSIX(_ string) error {
	return nil
}

// checkPOSIXExtended checks that no component has a leading hyphen.
// R1.3: rejects pathnames with leading-hyphen components.
// Stub: returns nil (success). Implementation fills in real logic.
func checkPOSIXExtended(_ string) error {
	return nil
}
