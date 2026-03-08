// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the mkdir utility for creating directories.
//
// Implements prd034-mkdir: basic directory creation (R1), parent directory
// creation (R2), mode setting (R3), differential testing (R4).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// flags holds the parsed command-line options.
type flags struct {
	parents bool   // -p, --parents: create parent directories as needed
	verbose bool   // -v, --verbose: print a message for each created directory
	mode    string // -m, --mode: file permission bits (octal or symbolic)
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, dirs := parseArgs(os.Args[1:])

	if len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "mkdir: missing operand\nTry 'mkdir --help' for more information.\n")
		os.Exit(1)
	}

	exitCode := 0
	for _, dir := range dirs {
		if err := createDir(dir, f); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and directory names.
func parseArgs(args []string) (flags, []string) {
	var f flags
	var dirs []string
	endFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endFlags || (len(arg) == 0) || arg[0] != '-' || arg == "-" {
			dirs = append(dirs, arg)
			continue
		}

		if arg == "--" {
			endFlags = true
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--parents":
				f.parents = true
			case arg == "--verbose":
				f.verbose = true
			case arg == "--mode":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "mkdir: option '--mode' requires an argument\n")
					os.Exit(1)
				}
				f.mode = args[i]
			case strings.HasPrefix(arg, "--mode="):
				f.mode = arg[len("--mode="):]
			default:
				fmt.Fprintf(os.Stderr, "mkdir: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}

		// Short options.
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'p':
				f.parents = true
			case 'v':
				f.verbose = true
			case 'm':
				// Mode value is the rest of this arg or the next arg.
				if j+1 < len(arg) {
					f.mode = arg[j+1:]
				} else {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "mkdir: option requires an argument -- 'm'\n")
						os.Exit(1)
					}
					f.mode = args[i]
				}
				j = len(arg) // break inner loop
			default:
				fmt.Fprintf(os.Stderr, "mkdir: invalid option -- '%c'\n", arg[j])
				os.Exit(1)
			}
		}
	}

	return f, dirs
}

// createDir creates a single directory, handling -p, -m, and -v flags.
func createDir(path string, f flags) error {
	perm := os.FileMode(0777)
	hasModeFlag := f.mode != ""

	if hasModeFlag {
		m, err := parseMode(f.mode)
		if err != nil {
			return err
		}
		perm = m
	}

	if f.parents {
		return createWithParents(path, perm, hasModeFlag, f.verbose)
	}

	if err := os.Mkdir(path, perm); err != nil {
		return formatMkdirError(path, err)
	}

	// R3.1: os.Mkdir applies umask; chmod to exact mode when -m is given.
	if hasModeFlag {
		if err := os.Chmod(path, perm); err != nil {
			return formatMkdirError(path, err)
		}
	}

	if f.verbose {
		fmt.Printf("mkdir: created directory '%s'\n", path)
	}

	return nil
}

// createWithParents implements -p: creates intermediate parent directories
// as needed. R2.1, R2.2, R2.3.
// R3.3: when -m is combined with -p, only the final target gets the specified
// mode; intermediate directories get default permissions.
func createWithParents(target string, perm os.FileMode, hasModeFlag, verbose bool) error {
	target = filepath.Clean(target)

	// Walk up from target to find the first existing ancestor.
	var toCreate []string
	current := target
	for {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("mkdir: cannot create directory '%s': Not a directory", target)
			}
			break
		}
		if !os.IsNotExist(err) {
			return formatMkdirError(current, err)
		}
		toCreate = append([]string{current}, toCreate...)
		parent := filepath.Dir(current)
		if parent == current {
			break // reached root
		}
		current = parent
	}

	for i, dir := range toCreate {
		if err := os.Mkdir(dir, 0777); err != nil {
			if os.IsExist(err) {
				continue // race condition or already exists
			}
			return formatMkdirError(dir, err)
		}
		// R3.3: apply explicit mode only to the final target.
		isFinal := i == len(toCreate)-1
		if isFinal && hasModeFlag {
			if err := os.Chmod(dir, perm); err != nil {
				return formatMkdirError(dir, err)
			}
		}
		if verbose {
			fmt.Printf("mkdir: created directory '%s'\n", dir)
		}
	}

	return nil
}

// parseMode parses a mode string as octal or symbolic notation. R3.1.
func parseMode(s string) (os.FileMode, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("mkdir: invalid mode '%s'", s)
	}

	// Try octal (digits 0-7 only).
	if s[0] >= '0' && s[0] <= '9' {
		n, err := strconv.ParseUint(s, 8, 32)
		if err != nil {
			return 0, fmt.Errorf("mkdir: invalid mode '%s'", s)
		}
		return os.FileMode(n), nil
	}

	// Symbolic mode.
	return parseSymbolicMode(s)
}

// parseSymbolicMode parses a symbolic mode string like "u=rwx,go=rx".
// The base mode for directories is 0777; symbolic clauses modify it.
func parseSymbolicMode(s string) (os.FileMode, error) {
	mode := os.FileMode(0777)
	clauses := strings.Split(s, ",")

	for _, clause := range clauses {
		if len(clause) == 0 {
			return 0, fmt.Errorf("mkdir: invalid mode '%s'", s)
		}

		i := 0
		// Parse who: u, g, o, a.
		var userMask, groupMask, otherMask bool
		for i < len(clause) {
			switch clause[i] {
			case 'u':
				userMask = true
			case 'g':
				groupMask = true
			case 'o':
				otherMask = true
			case 'a':
				userMask = true
				groupMask = true
				otherMask = true
			default:
				goto parseOp
			}
			i++
		}

	parseOp:
		// Default to all if no who specified.
		if !userMask && !groupMask && !otherMask {
			userMask = true
			groupMask = true
			otherMask = true
		}

		if i >= len(clause) {
			return 0, fmt.Errorf("mkdir: invalid mode '%s'", s)
		}

		op := clause[i]
		if op != '+' && op != '-' && op != '=' {
			return 0, fmt.Errorf("mkdir: invalid mode '%s'", s)
		}
		i++

		// Parse permission bits.
		var bits os.FileMode
		for i < len(clause) {
			switch clause[i] {
			case 'r':
				bits |= 4
			case 'w':
				bits |= 2
			case 'x':
				bits |= 1
			case 's':
				// setuid/setgid — not commonly used with mkdir, skip value
			case 't':
				// sticky bit — not commonly used with mkdir, skip value
			default:
				return 0, fmt.Errorf("mkdir: invalid mode '%s'", s)
			}
			i++
		}

		// Build the permission mask.
		var mask os.FileMode
		if userMask {
			mask |= bits << 6
		}
		if groupMask {
			mask |= bits << 3
		}
		if otherMask {
			mask |= bits
		}

		switch op {
		case '=':
			var clearMask os.FileMode
			if userMask {
				clearMask |= 0700
			}
			if groupMask {
				clearMask |= 0070
			}
			if otherMask {
				clearMask |= 0007
			}
			mode = (mode &^ clearMask) | mask
		case '+':
			mode |= mask
		case '-':
			mode &^= mask
		}
	}

	return mode, nil
}

// formatMkdirError formats an os error into GNU mkdir's error format.
func formatMkdirError(path string, err error) error {
	reason := err.Error()
	if pathErr, ok := err.(*os.PathError); ok {
		reason = capitalizeFirst(pathErr.Err.Error())
	}
	return fmt.Errorf("mkdir: cannot create directory '%s': %s", path, reason)
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
