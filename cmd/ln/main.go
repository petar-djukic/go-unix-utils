// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd037-ln R1.1-R1.4, R2.1-R2.4, R3.1-R3.6:
// cmd/ln creates hard or symbolic links between files. Supports two-argument form
// (ln TARGET LINK_NAME), single-argument form (ln TARGET), and
// multi-target form (ln TARGET... DIRECTORY). Supports -s (symbolic), -f (force),
// -n (no-dereference), -r (relative), -b/--backup (backup), -S/--suffix (backup suffix),
// -t/--target-directory, -T/--no-target-directory, and -v (verbose) flags.
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU ln format.
const progName = "ln"

// lnOptions holds the parsed flags for an ln invocation.
type lnOptions struct {
	symbolic        bool   // -s/--symbolic
	force           bool   // -f/--force
	noDereference   bool   // -n/--no-dereference
	verbose         bool   // -v/--verbose
	interactive     bool   // -i/--interactive
	backup          bool   // -b
	backupMethod    string // --backup=METHOD
	suffix          string // -S/--suffix
	targetDirectory string // -t/--target-directory
	noTargetDir     bool   // -T/--no-target-directory
	relative        bool   // -r/--relative
	showVersion     bool
	showHelp        bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, operands := parseArgs(os.Args[1:])

	if opts.showVersion {
		fmt.Println("ln (go-unix-utils) 0.1")
		os.Exit(0)
	}

	if opts.showHelp {
		printUsage(os.Stdout)
		os.Exit(0)
	}

	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	exitCode := 0

	// Determine the invocation form.
	if opts.targetDirectory != "" {
		// -t DIRECTORY form: all operands are targets.
		for _, target := range operands {
			linkName := filepath.Join(opts.targetDirectory, filepath.Base(target))
			if err := createLink(opts, target, linkName); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				exitCode = 1
			}
		}
	} else if opts.noTargetDir {
		// -T: treat last operand as a normal file name, never as a directory.
		if len(operands) != 2 {
			if len(operands) < 2 {
				fmt.Fprintf(os.Stderr, "%s: missing file operand\n", progName)
			} else {
				fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, operands[2])
			}
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
			os.Exit(1)
		}
		if err := createLink(opts, operands[0], operands[1]); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
	} else if len(operands) == 1 {
		// R1.2: single-argument form — create link in current directory.
		target := operands[0]
		linkName := filepath.Base(target)
		if err := createLink(opts, target, linkName); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 1
		}
	} else if len(operands) == 2 {
		// Check if last operand is a directory.
		// R2.3: with -n (no-dereference), use Lstat so a symlink to a directory
		// is treated as a regular file rather than followed.
		last := operands[1]
		var fi os.FileInfo
		var statErr error
		if opts.noDereference {
			fi, statErr = os.Lstat(last)
		} else {
			fi, statErr = os.Stat(last)
		}
		if statErr == nil && fi.IsDir() {
			// Multi-target into directory form with single target.
			target := operands[0]
			linkName := filepath.Join(last, filepath.Base(target))
			if err := createLink(opts, target, linkName); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				exitCode = 1
			}
		} else {
			// R1.1: two-argument form — ln TARGET LINK_NAME.
			if err := createLink(opts, operands[0], operands[1]); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				exitCode = 1
			}
		}
	} else {
		// R1.3: multi-target form — last operand must be a directory.
		dir := operands[len(operands)-1]
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			fmt.Fprintf(os.Stderr, "%s: target '%s': Not a directory\n", progName, dir)
			os.Exit(1)
		}
		for _, target := range operands[:len(operands)-1] {
			linkName := filepath.Join(dir, filepath.Base(target))
			if err := createLink(opts, target, linkName); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

// createLink creates a hard or symbolic link from target to linkName.
// R1.1: hard link via os.Link. R1.4: error if destination exists without -f.
// R3.5: backup existing destination when -b/--backup is set.
func createLink(opts *lnOptions, target, linkName string) error {
	// Check if destination exists.
	fi, err := os.Lstat(linkName)
	destExists := err == nil
	backupPath := ""
	if destExists {
		// Cannot overwrite a directory.
		if fi.IsDir() {
			return fmt.Errorf("%s: cannot overwrite directory", linkName)
		}
		if !opts.force && !opts.backup {
			linkType := "hard link"
			if opts.symbolic {
				linkType = "symbolic link"
			}
			return fmt.Errorf("failed to create %s '%s': File exists", linkType, linkName)
		}
		// R3.5: create backup before removal when -b/--backup is set.
		if opts.backup {
			backupPath = computeBackupPath(linkName, opts.backupMethod, opts.suffix)
		}
		if backupPath != "" {
			if err := os.Rename(linkName, backupPath); err != nil {
				return fmt.Errorf("cannot backup '%s': %v", linkName, unwrapPathError(err))
			}
		} else {
			// -f or --backup=none: remove existing destination before creating link.
			if err := os.Remove(linkName); err != nil {
				return fmt.Errorf("cannot remove '%s': %v", linkName, unwrapPathError(err))
			}
		}
	}

	// R1.3 (prd037): hard links to directories are not allowed.
	if !opts.symbolic {
		fi, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("failed to access '%s': %v", target, unwrapPathError(err))
		}
		if fi.IsDir() {
			return fmt.Errorf("hard link not allowed for directory '%s'", target)
		}
	}

	if opts.symbolic {
		// R2.4: compute relative path from link location to target when -r is set.
		symTarget := target
		if opts.relative {
			linkDir := filepath.Dir(linkName)
			absTarget, absErr := filepath.Abs(target)
			if absErr != nil {
				return fmt.Errorf("failed to resolve '%s': %v", target, absErr)
			}
			absLinkDir, absErr := filepath.Abs(linkDir)
			if absErr != nil {
				return fmt.Errorf("failed to resolve '%s': %v", linkDir, absErr)
			}
			rel, relErr := filepath.Rel(absLinkDir, absTarget)
			if relErr != nil {
				return fmt.Errorf("failed to compute relative path: %v", relErr)
			}
			symTarget = rel
		}
		if err := os.Symlink(symTarget, linkName); err != nil {
			return fmt.Errorf("failed to create symbolic link '%s': %v", linkName, unwrapPathError(err))
		}
	} else {
		if err := os.Link(target, linkName); err != nil {
			return fmt.Errorf("failed to create hard link '%s' => '%s': %v", linkName, target, unwrapPathError(err))
		}
	}

	if opts.verbose {
		arrow := "=>"
		if opts.symbolic {
			arrow = "->"
		}
		if backupPath != "" {
			fmt.Fprintf(os.Stdout, "'%s' ~ '%s' %s '%s'\n", backupPath, linkName, arrow, target)
		} else {
			fmt.Fprintf(os.Stdout, "'%s' %s '%s'\n", linkName, arrow, target)
		}
	}

	return nil
}

// computeBackupPath returns the backup file path based on the backup method.
// R3.5: simple appends suffix, numbered uses .~N~ format, existing checks for
// existing numbered backups and uses numbered if found, simple otherwise.
func computeBackupPath(linkName, method, suffix string) string {
	switch method {
	case "numbered", "t":
		return nextNumberedBackup(linkName)
	case "simple", "never":
		return linkName + suffix
	case "none", "off":
		return ""
	default:
		// "existing" or "nil": check if numbered backups already exist.
		if hasNumberedBackups(linkName) {
			return nextNumberedBackup(linkName)
		}
		return linkName + suffix
	}
}

// nextNumberedBackup finds the next available .~N~ backup path.
func nextNumberedBackup(linkName string) string {
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s.~%d~", linkName, n)
		if _, err := os.Lstat(candidate); err != nil {
			return candidate
		}
	}
}

// hasNumberedBackups checks if any .~N~ numbered backup files exist.
func hasNumberedBackups(linkName string) bool {
	// Check for .~1~ through a reasonable range.
	for n := 1; n <= 99; n++ {
		candidate := fmt.Sprintf("%s.~%d~", linkName, n)
		if _, err := os.Lstat(candidate); err == nil {
			return true
		}
	}
	return false
}

// parseArgs separates flags from operand arguments.
// R1.4: parses all GNU ln flags.
func parseArgs(args []string) (*lnOptions, []string) {
	opts := &lnOptions{
		suffix: "~", // default backup suffix
	}
	var operands []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--symbolic":
				opts.symbolic = true
			case arg == "--force":
				opts.force = true
			case arg == "--no-dereference":
				opts.noDereference = true
			case arg == "--verbose":
				opts.verbose = true
			case arg == "--interactive":
				opts.interactive = true
			case arg == "--backup":
				opts.backup = true
				opts.backupMethod = "existing"
			case strings.HasPrefix(arg, "--backup="):
				opts.backup = true
				opts.backupMethod = arg[len("--backup="):]
			case arg == "--suffix":
				if i+1 < len(args) {
					i++
					opts.suffix = args[i]
				}
			case strings.HasPrefix(arg, "--suffix="):
				opts.suffix = arg[len("--suffix="):]
			case arg == "--target-directory":
				if i+1 < len(args) {
					i++
					opts.targetDirectory = args[i]
				}
			case strings.HasPrefix(arg, "--target-directory="):
				opts.targetDirectory = arg[len("--target-directory="):]
			case arg == "--no-target-directory":
				opts.noTargetDir = true
			case arg == "--relative":
				opts.relative = true
			case arg == "--version":
				opts.showVersion = true
			case arg == "--help":
				opts.showHelp = true
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
			continue
		}
		// Short options.
		if arg[0] == '-' && len(arg) > 1 {
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 's':
					opts.symbolic = true
				case 'f':
					opts.force = true
				case 'n':
					opts.noDereference = true
				case 'v':
					opts.verbose = true
				case 'i':
					opts.interactive = true
				case 'b':
					opts.backup = true
					opts.backupMethod = "existing"
				case 'r':
					opts.relative = true
				case 'S':
					// -S consumes rest of arg or next arg.
					rest := arg[j+1:]
					if rest != "" {
						opts.suffix = rest
					} else if i+1 < len(args) {
						i++
						opts.suffix = args[i]
					}
					j = len(arg)
				case 't':
					// -t consumes rest of arg or next arg.
					rest := arg[j+1:]
					if rest != "" {
						opts.targetDirectory = rest
					} else if i+1 < len(args) {
						i++
						opts.targetDirectory = args[i]
					}
					j = len(arg)
				case 'T':
					opts.noTargetDir = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
					os.Exit(1)
				}
			}
			continue
		}
		operands = append(operands, arg)
	}
	return opts, operands
}

// unwrapPathError extracts the inner error from an *os.PathError or *os.LinkError.
// R4.3: ensures cross-device hard link errors and other OS errors are reported
// with the correct message format matching GNU ln.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	if le, ok := err.(*os.LinkError); ok {
		return le.Err
	}
	return err
}

// printUsage writes the help text to the given writer.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: ln [OPTION]... [-T] TARGET LINK_NAME")
	fmt.Fprintln(w, "  or:  ln [OPTION]... TARGET")
	fmt.Fprintln(w, "  or:  ln [OPTION]... TARGET... DIRECTORY")
	fmt.Fprintln(w, "  or:  ln [OPTION]... -t DIRECTORY TARGET...")
	fmt.Fprintln(w, "Create links between files.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  -b                         like --backup but does not accept an argument")
	fmt.Fprintln(w, "      --backup[=CONTROL]     make a backup of each existing destination file")
	fmt.Fprintln(w, "  -f, --force                remove existing destination files")
	fmt.Fprintln(w, "  -i, --interactive          prompt whether to remove destinations")
	fmt.Fprintln(w, "  -n, --no-dereference       treat LINK_NAME as a normal file if")
	fmt.Fprintln(w, "                               it is a symbolic link to a directory")
	fmt.Fprintln(w, "  -r, --relative             create symbolic links relative to link location")
	fmt.Fprintln(w, "  -s, --symbolic             make symbolic links instead of hard links")
	fmt.Fprintln(w, "  -S, --suffix=SUFFIX        override the usual backup suffix")
	fmt.Fprintln(w, "  -t, --target-directory=DIRECTORY  specify the DIRECTORY in which to create")
	fmt.Fprintln(w, "                               the links")
	fmt.Fprintln(w, "  -T, --no-target-directory   treat LINK_NAME as a normal file always")
	fmt.Fprintln(w, "  -v, --verbose              print name of each linked file")
	fmt.Fprintln(w, "      --help                 display this help and exit")
	fmt.Fprintln(w, "      --version              output version information and exit")
}
