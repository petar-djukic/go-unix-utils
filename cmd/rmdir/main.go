// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd035-rmdir R1.1-R1.4:
// cmd/rmdir removes empty directories. Supports --ignore-fail-on-non-empty
// to suppress errors for non-empty directories and -p/--parents to remove
// each directory component in the path. Installs SIGPIPE handler for clean
// exit on broken pipe.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in diagnostic output.
const progName = "rmdir"

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var parents bool
	var ignoreNonEmpty bool

	// Parse flags manually to match GNU rmdir behavior.
	var operands []string
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if arg == "--help" {
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s [OPTION]... DIRECTORY...\n"+
					"Remove the DIRECTORY(ies), if they are empty.\n\n"+
					"      --ignore-fail-on-non-empty\n"+
					"                  ignore each failure that is solely because a directory\n"+
					"                  is non-empty\n"+
					"  -p, --parents   remove DIRECTORY and its ancestors; e.g., 'rmdir -p a/b/c' is\n"+
					"                  similar to 'rmdir a/b/c a/b a'\n"+
					"  -v, --verbose   output a diagnostic for every directory processed\n"+
					"      --help      display this help and exit\n"+
					"      --version   output version information and exit\n",
				progName,
			)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}
		if arg == "--parents" {
			parents = true
			continue
		}
		if arg == "--ignore-fail-on-non-empty" {
			ignoreNonEmpty = true
			continue
		}
		// Reject unrecognized long options.
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)     //nolint:errcheck // best-effort diagnostic
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
			os.Exit(1)
		}
		// Handle short flags, possibly combined (e.g., -p).
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'p':
					parents = true
					j++
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, arg[j])    //nolint:errcheck // best-effort diagnostic
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
					os.Exit(1)
				}
			}
			continue
		}
		operands = append(operands, arg)
	}

	// R1.2: exit 1 with diagnostic when no operands given.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)                   //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	exitCode := 0
	for _, dir := range operands {
		if err := removeDir(dir, parents, ignoreNonEmpty); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err) //nolint:errcheck // best-effort diagnostic
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// removeDir removes the directory and optionally its parent components.
// R1.1: remove a single empty directory.
// R1.2: report errors for non-empty, non-existent, or non-directory targets.
// R1.3: --ignore-fail-on-non-empty suppresses ENOTEMPTY errors.
// R1.4: -p removes parent directory components in sequence.
func removeDir(dir string, parents, ignoreNonEmpty bool) error {
	if err := tryRemove(dir, ignoreNonEmpty); err != nil {
		return err
	}

	if parents {
		// R1.4: remove each parent component.
		current := dir
		for {
			current = filepath.Dir(current)
			if current == "." || current == "/" {
				break
			}
			if err := tryRemove(current, ignoreNonEmpty); err != nil {
				return err
			}
		}
	}

	return nil
}

// tryRemove attempts to remove a single directory. If ignoreNonEmpty is true,
// errors caused by a non-empty directory are suppressed.
func tryRemove(dir string, ignoreNonEmpty bool) error {
	err := syscall.Rmdir(dir)
	if err == nil {
		return nil
	}

	// R1.3: suppress ENOTEMPTY when --ignore-fail-on-non-empty is set.
	if ignoreNonEmpty && isDirNotEmpty(err) {
		return nil
	}

	return fmt.Errorf("failed to remove '%s': %s", dir, err)
}

// isDirNotEmpty checks if the error indicates a non-empty directory.
func isDirNotEmpty(err error) bool {
	return err == syscall.ENOTEMPTY
}
