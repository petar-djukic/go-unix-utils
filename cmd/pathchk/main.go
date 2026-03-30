// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pathchk checks whether file names are valid or portable (prd103-pathchk R1, R2).
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName     = "pathchk"
	posixNameMax = 14
	posixPathMax = 256
	sysNameMax   = 255
	sysPathMax   = 1024
)

const versionStr = progName + " (go-unix-utils) 1.0"

const helpStr = `Usage: pathchk [OPTION]... NAME...
Diagnose invalid or unportable file names.

  -p                  check for most POSIX systems
  -P                  check for empty names and leading "-"
      --portability   check for all POSIX systems (equivalent to -p -P)
      --help          display this help and exit
      --version       output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses flags, iterates over pathname arguments, and dispatches checks.
// R2.1: exits 0 when all pathnames pass. R2.2: exits 1 when any fails.
func run(args []string, stdout, stderr *os.File) int {
	posix, extended, paths, done := parseArgs(args, stdout)
	if done {
		return 0
	}
	if len(paths) == 0 {
		return 0
	}
	exitCode := 0
	checkBasic := !posix && !extended
	for _, path := range paths {
		if reportErrors(path, checkBasic, posix, extended, stderr) {
			exitCode = 1
		}
	}
	return exitCode
}

// reportErrors runs applicable checks on path, printing errors to stderr.
// D3: all modes are checked independently; multiple errors may be reported.
func reportErrors(path string, basic, posix, extended bool, w *os.File) bool {
	hadError := false
	if basic {
		if err := checkDefault(path); err != nil {
			fmt.Fprintln(w, err)
			hadError = true
		}
	}
	if posix {
		if err := checkPOSIX(path); err != nil {
			fmt.Fprintln(w, err)
			hadError = true
		}
	}
	if extended {
		if err := checkPOSIXExtended(path); err != nil {
			fmt.Fprintln(w, err)
			hadError = true
		}
	}
	return hadError
}

// parseArgs extracts -p, -P, --portability, --help, --version flags
// and remaining pathnames. R1.4: supports multiple pathname arguments.
// R2.3: --help prints usage and returns done=true, --version prints version.
func parseArgs(args []string, stdout *os.File) (posix, extended bool, paths []string, done bool) {
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "--portability" {
			posix = true
			extended = true
			continue
		}
		if arg == "--help" {
			fmt.Fprint(stdout, helpStr)
			return false, false, nil, true
		}
		if arg == "--version" {
			fmt.Fprintln(stdout, versionStr)
			return false, false, nil, true
		}
		if isKnownFlags(arg) {
			posix, extended = applyFlags(arg, posix, extended)
			continue
		}
		paths = append(paths, arg)
	}
	return posix, extended, paths, false
}

// isKnownFlags returns true if arg is a short flag group containing only p and P.
func isKnownFlags(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	for i := 1; i < len(arg); i++ {
		if arg[i] != 'p' && arg[i] != 'P' {
			return false
		}
	}
	return true
}

// applyFlags sets posix and extended flags from a short flag group.
func applyFlags(arg string, posix, extended bool) (bool, bool) {
	for i := 1; i < len(arg); i++ {
		switch arg[i] {
		case 'p':
			posix = true
		case 'P':
			extended = true
		}
	}
	return posix, extended
}

// checkDefault checks a pathname against system limits.
// R1.1: checks component length against NAME_MAX, total path against PATH_MAX.
func checkDefault(path string) error {
	if path == "" {
		return fmt.Errorf("%s: empty file name", progName)
	}
	if len(path) >= sysPathMax {
		return fmt.Errorf("%s: limit %d exceeded by length %d of file name '%s'",
			progName, sysPathMax-1, len(path), path)
	}
	return checkDefaultComponents(path)
}

// checkDefaultComponents checks component lengths and intermediate directory existence.
// R1.1: verifies each component exists or could be created.
func checkDefaultComponents(path string) error {
	absolute := strings.HasPrefix(path, "/")
	components := splitNonEmpty(path)
	var prefix string
	for i, comp := range components {
		if len(comp) > sysNameMax {
			return fmtComponentTooLong(sysNameMax, len(comp), comp)
		}
		prefix = growPrefix(prefix, comp, absolute && prefix == "")
		if i < len(components)-1 {
			if _, err := os.Stat(prefix); err != nil {
				return fmtStatError(prefix, err)
			}
		}
	}
	return nil
}

// checkPOSIX checks a pathname against POSIX portability rules.
// R1.2: only portable filename characters, component <= 14, total <= 256.
func checkPOSIX(path string) error {
	if path == "" {
		return fmt.Errorf("%s: empty file name", progName)
	}
	if len(path) >= posixPathMax {
		return fmt.Errorf("%s: limit %d exceeded by length %d of file name '%s'",
			progName, posixPathMax-1, len(path), path)
	}
	return checkPOSIXComponents(path)
}

// checkPOSIXComponents checks each component for portable characters and length.
func checkPOSIXComponents(path string) error {
	for _, comp := range splitNonEmpty(path) {
		if err := checkPortableChars(comp, path); err != nil {
			return err
		}
		if len(comp) > posixNameMax {
			return fmtComponentTooLong(posixNameMax, len(comp), comp)
		}
	}
	return nil
}

// checkPortableChars returns an error if comp contains a non-portable character.
// R1.2: portable set is A-Z, a-z, 0-9, period, underscore, hyphen.
func checkPortableChars(comp, path string) error {
	for i := 0; i < len(comp); i++ {
		if !isPortableChar(comp[i]) {
			return fmt.Errorf("%s: nonportable character '%c' in file name '%s'",
				progName, comp[i], path)
		}
	}
	return nil
}

// isPortableChar returns true if c is in the POSIX portable filename character set.
func isPortableChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
}

// checkPOSIXExtended checks a pathname for extended POSIX portability.
// R2.1: rejects empty pathnames, empty components, and leading hyphens.
func checkPOSIXExtended(path string) error {
	if path == "" {
		return fmt.Errorf("%s: empty file name", progName)
	}
	if err := checkEmptyComponents(path); err != nil {
		return err
	}
	return checkLeadingHyphen(path)
}

// checkEmptyComponents rejects consecutive slashes, leading slash, or
// trailing slash that produce empty pathname components.
// R2.1: empty component check for -P mode.
func checkEmptyComponents(path string) error {
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if p == "" {
			return fmt.Errorf("%s: empty file name component in file name '%s'",
				progName, path)
		}
	}
	return nil
}

// checkLeadingHyphen checks that no component starts with a hyphen.
// R1.3: rejects pathnames with leading-hyphen components.
func checkLeadingHyphen(path string) error {
	for _, comp := range splitNonEmpty(path) {
		if len(comp) > 0 && comp[0] == '-' {
			return fmt.Errorf("%s: leading '-' in a component of file name '%s'",
				progName, path)
		}
	}
	return nil
}

// splitNonEmpty splits path by "/" and returns only non-empty components.
func splitNonEmpty(path string) []string {
	parts := strings.Split(path, "/")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// growPrefix appends comp to the current path prefix.
func growPrefix(prefix, comp string, leadingSlash bool) string {
	if leadingSlash {
		return "/" + comp
	}
	if prefix == "" {
		return comp
	}
	return prefix + "/" + comp
}

// fmtComponentTooLong formats a component-too-long error.
func fmtComponentTooLong(limit, length int, comp string) error {
	return fmt.Errorf("%s: limit %d exceeded by length %d of file name component '%s'",
		progName, limit, length, comp)
}

// fmtStatError formats an error from os.Stat with a capitalized errno message.
func fmtStatError(prefix string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		msg := pe.Err.Error()
		if len(msg) > 0 && msg[0] >= 'a' && msg[0] <= 'z' {
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
		return fmt.Errorf("%s: '%s': %s", progName, prefix, msg)
	}
	return fmt.Errorf("%s: '%s': %s", progName, prefix, err)
}
