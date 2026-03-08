// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ln utility for creating links between files.
//
// Implements prd037-ln: hard link creation (R1), symbolic link creation (R2),
// force/safety/verbosity flags (R3), differential testing (R4).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// flags holds the parsed command-line options.
type flags struct {
	symbolic     bool   // -s, --symbolic: create symbolic links
	force        bool   // -f, --force: remove existing destinations
	noDereference bool  // -n, --no-dereference: treat symlink-to-dir as regular file
	interactive  bool   // -i, --interactive: prompt before removing
	verbose      bool   // -v, --verbose: print name of each link
	backup       bool   // -b: create backup with default suffix
	backupMethod string // --backup=METHOD: numbered, existing, simple, none
	suffix       string // -S, --suffix: backup suffix (default "~")
	relative     bool   // -r, --relative: create relative symbolic links
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, operands := parseArgs(os.Args[1:])

	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "ln: missing file operand\nTry 'ln --help' for more information.\n")
		os.Exit(1)
	}

	if len(operands) == 1 {
		// ln TARGET: create link in current directory with same basename.
		target := operands[0]
		linkName := filepath.Base(target)
		if err := createLink(target, linkName, f); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Two or more operands.
	dest := operands[len(operands)-1]
	sources := operands[:len(operands)-1]

	if len(sources) == 1 {
		// ln TARGET LINK_NAME — or — ln TARGET DIRECTORY
		info, err := os.Stat(dest)
		if err == nil && info.IsDir() {
			// Destination is a directory: create link inside it.
			exitCode := 0
			for _, src := range sources {
				linkPath := filepath.Join(dest, filepath.Base(src))
				if err := createLink(src, linkPath, f); err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", err)
					exitCode = 1
				}
			}
			os.Exit(exitCode)
		}
		// Destination is not a directory: treat as link name.
		if err := createLink(sources[0], dest, f); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Multiple sources: dest must be a directory. R1.2.
	info, err := os.Stat(dest)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "ln: target '%s': Not a directory\n", dest)
		os.Exit(1)
	}

	exitCode := 0
	for _, src := range sources {
		linkPath := filepath.Join(dest, filepath.Base(src))
		if err := createLink(src, linkPath, f); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and operands.
func parseArgs(args []string) (flags, []string) {
	var f flags
	f.suffix = "~" // default backup suffix
	var operands []string
	endFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endFlags || len(arg) == 0 || arg[0] != '-' || arg == "-" {
			operands = append(operands, arg)
			continue
		}

		if arg == "--" {
			endFlags = true
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--symbolic":
				f.symbolic = true
			case arg == "--force":
				f.force = true
			case arg == "--no-dereference":
				f.noDereference = true
			case arg == "--interactive":
				f.interactive = true
			case arg == "--verbose":
				f.verbose = true
			case arg == "--relative":
				f.relative = true
			case arg == "--backup":
				f.backup = true
				f.backupMethod = "existing"
			case strings.HasPrefix(arg, "--backup="):
				f.backup = true
				f.backupMethod = arg[len("--backup="):]
			case arg == "--suffix":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "ln: option '--suffix' requires an argument\n")
					os.Exit(1)
				}
				f.suffix = args[i]
			case strings.HasPrefix(arg, "--suffix="):
				f.suffix = arg[len("--suffix="):]
			default:
				fmt.Fprintf(os.Stderr, "ln: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}

		// Short options.
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 's':
				f.symbolic = true
			case 'f':
				f.force = true
				f.interactive = false // -f overrides -i
			case 'n':
				f.noDereference = true
			case 'i':
				f.interactive = true
				f.force = false // -i overrides -f
			case 'v':
				f.verbose = true
			case 'b':
				f.backup = true
				f.backupMethod = "existing"
			case 'r':
				f.relative = true
			case 'S':
				if j+1 < len(arg) {
					f.suffix = arg[j+1:]
				} else {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "ln: option requires an argument -- 'S'\n")
						os.Exit(1)
					}
					f.suffix = args[i]
				}
				j = len(arg) // break inner loop
			default:
				fmt.Fprintf(os.Stderr, "ln: invalid option -- '%c'\n", arg[j])
				os.Exit(1)
			}
		}
	}

	return f, operands
}

// createLink creates a single link, handling force, backup, verbose, and relative flags.
func createLink(target, linkName string, f flags) error {
	// R2.4: compute relative path for -r.
	actualTarget := target
	if f.symbolic && f.relative {
		linkDir := filepath.Dir(linkName)
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("ln: failed to resolve '%s': %s", target, err)
		}
		absLinkDir, err := filepath.Abs(linkDir)
		if err != nil {
			return fmt.Errorf("ln: failed to resolve '%s': %s", linkDir, err)
		}
		rel, err := filepath.Rel(absLinkDir, absTarget)
		if err != nil {
			return fmt.Errorf("ln: failed to make relative path: %s", err)
		}
		actualTarget = rel
	}

	// Check if destination exists.
	destExists := false
	var destInfo os.FileInfo
	if f.noDereference {
		// R3.2: use Lstat to not follow symlinks.
		info, err := os.Lstat(linkName)
		if err == nil {
			destExists = true
			destInfo = info
		}
	} else {
		info, err := os.Lstat(linkName)
		if err == nil {
			destExists = true
			destInfo = info
			// If it's a symlink to a directory, check what it points to.
			if destInfo.Mode()&os.ModeSymlink != 0 {
				resolved, statErr := os.Stat(linkName)
				if statErr == nil && resolved.IsDir() {
					// Destination is a symlink to a directory; create link inside.
					linkName = filepath.Join(linkName, filepath.Base(target))
					// Re-check existence at the new path.
					_, err := os.Lstat(linkName)
					destExists = err == nil
				}
			}
		}
	}

	if destExists {
		if !f.force && !f.interactive && !f.backup {
			return fmt.Errorf("ln: failed to create %s link '%s': File exists",
				linkTypeStr(f.symbolic), linkName)
		}

		// R3.3: interactive prompt.
		if f.interactive {
			fmt.Fprintf(os.Stderr, "ln: replace '%s'? ", linkName)
			var response string
			fmt.Scanln(&response)
			if len(response) == 0 || (response[0] != 'y' && response[0] != 'Y') {
				return nil
			}
		}

		// R3.5: create backup before removal.
		if f.backup {
			backupPath := computeBackupPath(linkName, f.suffix, f.backupMethod)
			if err := os.Rename(linkName, backupPath); err != nil {
				return fmt.Errorf("ln: cannot backup '%s': %s", linkName, err)
			}
		} else {
			// R3.1: remove existing destination.
			if err := os.Remove(linkName); err != nil {
				return formatLnError(linkName, err)
			}
		}
	}

	// Create the link.
	if f.symbolic {
		if err := os.Symlink(actualTarget, linkName); err != nil {
			return formatLnError(linkName, err)
		}
	} else {
		// R1.3: check for hard link to directory before attempting.
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("ln: failed to access '%s': %s", target, capitalizeFirst(unwrapErr(err)))
		}
		if info.IsDir() {
			return fmt.Errorf("ln: %s: hard link not allowed for directory", target)
		}
		if err := os.Link(target, linkName); err != nil {
			return formatLnError(linkName, err)
		}
	}

	// R3.4: verbose output.
	if f.verbose {
		if f.symbolic {
			fmt.Printf("'%s' -> '%s'\n", linkName, actualTarget)
		} else {
			fmt.Printf("'%s' => '%s'\n", linkName, target)
		}
	}

	return nil
}

// linkTypeStr returns "hard" or "symbolic" for error messages.
func linkTypeStr(symbolic bool) string {
	if symbolic {
		return "symbolic"
	}
	return "hard"
}

// computeBackupPath determines the backup file path based on method and suffix.
func computeBackupPath(path, suffix, method string) string {
	switch method {
	case "numbered", "t":
		// Find the next available numbered backup.
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s.~%d~", path, i)
			if _, err := os.Lstat(candidate); os.IsNotExist(err) {
				return candidate
			}
		}
	case "existing", "nil":
		// Use numbered if numbered backups already exist, otherwise simple.
		if _, err := os.Lstat(path + ".~1~"); err == nil {
			return computeBackupPath(path, suffix, "numbered")
		}
		return path + suffix
	case "simple", "never":
		return path + suffix
	case "none", "off":
		return path + suffix
	default:
		return path + suffix
	}
}

// formatLnError formats an os error into GNU ln's error format.
func formatLnError(path string, err error) error {
	reason := err.Error()
	if pathErr, ok := err.(*os.PathError); ok {
		reason = capitalizeFirst(pathErr.Err.Error())
	}
	if linkErr, ok := err.(*os.LinkError); ok {
		return fmt.Errorf("ln: failed to create %s link '%s' => '%s': %s",
			"hard", linkErr.New, linkErr.Old, capitalizeFirst(linkErr.Err.Error()))
	}
	return fmt.Errorf("ln: failed to create link '%s': %s", path, reason)
}

// unwrapErr extracts the inner error string from a path or link error.
func unwrapErr(err error) string {
	if pathErr, ok := err.(*os.PathError); ok {
		return pathErr.Err.Error()
	}
	if linkErr, ok := err.(*os.LinkError); ok {
		return linkErr.Err.Error()
	}
	return err.Error()
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
