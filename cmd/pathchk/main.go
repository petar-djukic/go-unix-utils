// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pathchk implements GNU pathchk: check whether file names are valid or portable.
// Implements prd103-pathchk R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName = "pathchk"

	// R1.2: POSIX portability limits for -p mode.
	posixNameMax = 14
	posixPathMax = 256

	// R1.1: system limits (common defaults for Darwin and Linux).
	sysNameMax = 255
	sysPathMax = 1024

	// R1.2: POSIX portable filename character set.
	portableSet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-"
)

// R2.3: SIGPIPE handling. R1.4: process multiple arguments. R2.1/R2.2: exit codes.
func main() {
	sys.InstallSIGPIPEHandler()
	flagP, flagBigP, args := parseArgs(os.Args[1:])
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\nTry '%s --help' for more information.\n",
			progName, progName)
		os.Exit(1)
	}
	exitCode := 0
	for _, path := range args {
		if !validatePath(path, flagP, flagBigP) {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// parseArgs extracts -p, -P, --portability flags and returns remaining operands.
// R1.2: --portability is the long form of -p.
func parseArgs(args []string) (flagP, flagBigP bool, operands []string) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "--portability" {
			flagP = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\nTry '%s --help' for more information.\n",
				progName, arg, progName)
			os.Exit(2)
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		parseShortFlags(arg[1:], &flagP, &flagBigP)
		i++
	}
	operands = args[i:]
	return
}

// parseShortFlags processes combined short flags like -pP.
func parseShortFlags(flags string, flagP, flagBigP *bool) {
	for _, ch := range flags {
		switch ch {
		case 'p':
			*flagP = true
		case 'P':
			*flagBigP = true
		default:
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\nTry '%s --help' for more information.\n",
				progName, ch, progName)
			os.Exit(2)
		}
	}
}

// validatePath runs all applicable checks on a single pathname.
// R1.4: processes each pathname argument independently.
func validatePath(path string, flagP, flagBigP bool) bool {
	if path == "" {
		fmt.Fprintf(os.Stderr, "%s: empty file name\n", progName)
		return false
	}
	ok := true
	if flagBigP && !checkExtra(path) {
		ok = false
	}
	if flagP {
		if !checkPortable(path) {
			ok = false
		}
	} else if !checkBasic(path) {
		ok = false
	}
	return ok
}

// checkBasic validates path against system NAME_MAX/PATH_MAX and leading dir existence.
// R1.1: system validity checks.
func checkBasic(path string) bool {
	ok := true
	if len(path) > sysPathMax {
		diagPathTooLong(path, sysPathMax)
		ok = false
	}
	if !checkComponentLengths(path, sysNameMax) {
		ok = false
	}
	if !checkLeadingDirs(path) {
		ok = false
	}
	return ok
}

// checkPortable validates path against POSIX portable filename rules.
// R1.2: POSIX portability checks with NAME_MAX=14, PATH_MAX=256.
func checkPortable(path string) bool {
	ok := true
	if len(path) > posixPathMax {
		diagPathTooLong(path, posixPathMax)
		ok = false
	}
	if !checkComponentLengths(path, posixNameMax) {
		ok = false
	}
	if !checkPortableChars(path) {
		ok = false
	}
	return ok
}

// checkExtra validates -P constraints: no leading hyphens in components.
// R1.3: reject leading hyphens.
func checkExtra(path string) bool {
	for _, comp := range strings.Split(path, "/") {
		if strings.HasPrefix(comp, "-") {
			fmt.Fprintf(os.Stderr, "%s: leading '-' in a component of file name '%s'\n",
				progName, path)
			return false
		}
	}
	return true
}

// checkComponentLengths checks each path component against the given name limit.
func checkComponentLengths(path string, limit int) bool {
	for _, comp := range strings.Split(path, "/") {
		if len(comp) > limit {
			fmt.Fprintf(os.Stderr,
				"%s: limit %d exceeded by length %d of file name component '%s'\n",
				progName, limit, len(comp), comp)
			return false
		}
	}
	return true
}

// checkPortableChars verifies every non-slash character is in the POSIX portable set.
func checkPortableChars(path string) bool {
	for _, ch := range path {
		if ch != '/' && !strings.ContainsRune(portableSet, ch) {
			fmt.Fprintf(os.Stderr, "%s: nonportable character '%c' in file name '%s'\n",
				progName, ch, path)
			return false
		}
	}
	return true
}

// checkLeadingDirs verifies that all leading directory components of path exist.
func checkLeadingDirs(path string) bool {
	dir := filepath.Dir(path)
	if dir == "." || dir == "/" {
		return true
	}
	if _, err := os.Stat(dir); err != nil {
		fmt.Fprintf(os.Stderr, "%s: '%s': No such file or directory\n", progName, path)
		return false
	}
	return true
}

// diagPathTooLong prints the diagnostic for path length exceeded.
func diagPathTooLong(path string, limit int) {
	fmt.Fprintf(os.Stderr, "%s: limit %d exceeded by length %d of file name '%s'\n",
		progName, limit, len(path), path)
}
