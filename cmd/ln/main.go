// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd037-ln R1.1–R1.4, R2.1–R2.4
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "ln"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	// R2.1: -s / --symbolic flag for symbolic link creation.
	symbolic := false
	// R2.4: -r / --relative flag for relative symlink paths.
	relative := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--symbolic":
			symbolic = true
		case arg == "--relative":
			relative = true
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options (e.g., -sr).
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 's':
					symbolic = true
				case 'r':
					relative = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, flags[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R2.4: -r requires -s; error if -r is given without -s, matching GNU behavior.
	if relative && !symbolic {
		fmt.Fprintf(os.Stderr, "%s: cannot do --relative without --symbolic\n", programName)
		os.Exit(1)
	}

	// R1.1: at least one operand is required.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	if len(operands) == 1 {
		// R1.1: ln TARGET creates a link in the current directory with the same basename.
		target := operands[0]
		linkName := filepath.Base(target)
		os.Exit(createLink(target, linkName, symbolic, relative))
	}

	// Check if the last operand is a directory for multi-target mode.
	last := operands[len(operands)-1]
	lastInfo, err := os.Stat(last)

	if len(operands) == 2 && (err != nil || !lastInfo.IsDir()) {
		// R1.1: ln TARGET LINK_NAME — two-operand form, last is not a directory.
		os.Exit(createLink(operands[0], operands[1], symbolic, relative))
	}

	// R1.2: ln TARGET... DIRECTORY — multiple targets into a directory.
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: target '%s': No such file or directory\n", programName, last)
		os.Exit(1)
	}
	if !lastInfo.IsDir() {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n", programName, last)
		os.Exit(1)
	}

	exitCode := 0
	targets := operands[:len(operands)-1]
	for _, target := range targets {
		linkName := filepath.Join(last, filepath.Base(target))
		if code := createLink(target, linkName, symbolic, relative); code != 0 {
			exitCode = code
		}
	}
	os.Exit(exitCode)
}

// createLink creates a hard or symbolic link from target to linkName.
// Returns 0 on success, 1 on failure.
//
// R1.1, R1.2: hard link creation.
// R1.3: error when hard linking to a directory.
// R1.4: error when destination already exists.
// R2.1: symbolic link creation with -s.
// R2.2: symbolic links to directories are allowed.
// R2.3: target string stored as-is in the symlink.
// R2.4: relative flag computes a relative path from link location to target.
func createLink(target, linkName string, symbolic, relative bool) int {
	// R1.4: check if destination already exists.
	if _, err := os.Lstat(linkName); err == nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create %s link '%s': File exists\n",
			programName, linkType(symbolic), linkName)
		return 1
	}

	if symbolic {
		symlinkTarget := target
		// R2.4: compute relative path from the link's directory to the target.
		if relative {
			relTarget, err := computeRelativeTarget(target, linkName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: failed to compute relative path from '%s' to '%s': %s\n",
					programName, linkName, target, err)
				return 1
			}
			symlinkTarget = relTarget
		}
		// R2.1, R2.2, R2.3: create symbolic link, storing target string as-is.
		if err := os.Symlink(symlinkTarget, linkName); err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to create symbolic link '%s': %s\n",
				programName, linkName, errMessage(err))
			return 1
		}
		return 0
	}

	// R1.3: error when hard linking to a directory.
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to access '%s': %s\n",
			programName, target, errMessage(err))
		return 1
	}
	if info.IsDir() {
		fmt.Fprintf(os.Stderr, "%s: %s: hard link not allowed for directory\n",
			programName, target)
		return 1
	}

	// D2: use os.Link for hard links.
	if err := os.Link(target, linkName); err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create hard link '%s' => '%s': %s\n",
			programName, linkName, target, errMessage(err))
		return 1
	}
	return 0
}

// linkType returns "hard" or "symbolic" for use in error messages.
func linkType(symbolic bool) string {
	if symbolic {
		return "symbolic"
	}
	return "hard"
}

// computeRelativeTarget computes a relative path from the directory containing
// linkName to the target path, matching GNU ln -r behavior.
//
// R2.4: both paths are resolved to absolute before calling filepath.Rel.
func computeRelativeTarget(target, linkName string) (string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolving target: %w", err)
	}
	absLink, err := filepath.Abs(linkName)
	if err != nil {
		return "", fmt.Errorf("resolving link: %w", err)
	}
	linkDir := filepath.Dir(absLink)
	rel, err := filepath.Rel(linkDir, absTarget)
	if err != nil {
		return "", fmt.Errorf("computing relative path: %w", err)
	}
	return rel, nil
}

// errMessage extracts the underlying error message from a *os.PathError,
// stripping the op and path prefix that Go adds, to match GNU coreutils style.
func errMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printHelp writes usage information to stdout and exits 0.
//
// R1.4: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: ln [OPTION]... [-T] TARGET LINK_NAME
  or:  ln [OPTION]... TARGET
  or:  ln [OPTION]... TARGET... DIRECTORY
Create a link to TARGET with the name LINK_NAME or in DIRECTORY.
Create a hard link by default, a symbolic link with --symbolic.

  -r, --relative  with -s, create relative symbolic links
  -s, --symbolic  make symbolic links instead of hard links
      --help      display this help and exit
      --version   output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
//
// R1.4: --version prints version info to stdout and exits 0.
func printVersion() {
	fmt.Println("ln (go-unix-utils) 0.1")
}
