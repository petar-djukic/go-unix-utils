// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd037-ln R1.1–R1.4, R2.1–R2.4, R3.1–R3.6
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "ln"

// defaultBackupSuffix is the default suffix appended to simple backup files.
// R3.6: overridden by -S/--suffix.
const defaultBackupSuffix = "~"

// Canonical backup method names matching GNU ln.
// R3.5: backup method constants.
const (
	backupNone     = "none"
	backupSimple   = "simple"
	backupNumbered = "numbered"
	backupExisting = "existing"
)

// linkOpts holds all flag state for a ln invocation.
type linkOpts struct {
	symbolic      bool
	relative      bool
	force         bool
	noDereference bool
	backup        bool
	backupMethod  string
	suffix        string
	verbose       bool
}

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	opts := linkOpts{
		suffix: defaultBackupSuffix,
	}
	backupRequested := false
	var backupMethodExplicit string

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
			opts.symbolic = true
		case arg == "--relative":
			opts.relative = true
		case arg == "--force":
			opts.force = true
		case arg == "--no-dereference":
			opts.noDereference = true
		case arg == "--verbose":
			opts.verbose = true
		case arg == "--backup":
			// R3.5: --backup with no argument uses VERSION_CONTROL or "existing".
			backupRequested = true
		case strings.HasPrefix(arg, "--backup="):
			// R3.5: --backup=METHOD sets method explicitly.
			backupRequested = true
			backupMethodExplicit = arg[len("--backup="):]
		case arg == "--suffix":
			// R3.6: --suffix SUFFIX (space-separated form).
			if i+1 < len(args) {
				i++
				opts.suffix = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "%s: option '--suffix' requires an argument\n", programName)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
				os.Exit(1)
			}
		case strings.HasPrefix(arg, "--suffix="):
			// R3.6: --suffix=SUFFIX sets backup suffix.
			opts.suffix = arg[len("--suffix="):]
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
			// Parse bundled short options (e.g., -sfb).
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 's':
					opts.symbolic = true
				case 'r':
					opts.relative = true
				case 'f':
					opts.force = true
				case 'n':
					opts.noDereference = true
				case 'v':
					opts.verbose = true
				case 'b':
					// R3.5: -b is shorthand for --backup (no explicit method).
					backupRequested = true
				case 'S':
					// R3.6: -S takes the rest of the bundle or the next argument as suffix.
					if j+1 < len(flags) {
						opts.suffix = flags[j+1:]
						j = len(flags)
					} else if i+1 < len(args) {
						i++
						opts.suffix = args[i]
					} else {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'S'\n", programName)
						fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
						os.Exit(1)
					}
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

	// Resolve backup method when backup was requested.
	if backupRequested {
		if backupMethodExplicit != "" {
			method, err := resolveBackupMethod(backupMethodExplicit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
				os.Exit(1)
			}
			if method != backupNone {
				opts.backup = true
				opts.backupMethod = method
			}
		} else {
			// R3.4: VERSION_CONTROL env var provides default method.
			if vc, ok := os.LookupEnv("VERSION_CONTROL"); ok {
				method, err := resolveBackupMethod(vc)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
				if method != backupNone {
					opts.backup = true
					opts.backupMethod = method
				}
			} else {
				opts.backup = true
				opts.backupMethod = backupExisting
			}
		}
	}

	// R2.4: -r requires -s; error if -r is given without -s, matching GNU behavior.
	if opts.relative && !opts.symbolic {
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
		os.Exit(createLink(target, linkName, opts))
	}

	// Check if the last operand is a directory for multi-target mode.
	// R3.2: when -n is set, use Lstat so symlinks to directories are not followed.
	last := operands[len(operands)-1]
	var lastInfo os.FileInfo
	var err error
	if opts.noDereference {
		lastInfo, err = os.Lstat(last)
	} else {
		lastInfo, err = os.Stat(last)
	}

	if len(operands) == 2 && (err != nil || !lastInfo.IsDir()) {
		// R1.1: ln TARGET LINK_NAME — two-operand form, last is not a directory.
		os.Exit(createLink(operands[0], operands[1], opts))
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
		if code := createLink(target, linkName, opts); code != 0 {
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
// R1.4: error when destination already exists (without -f or --backup).
// R2.1: symbolic link creation with -s.
// R2.2: symbolic links to directories are allowed.
// R2.3: target string stored as-is in the symlink.
// R2.4: relative flag computes a relative path from link location to target.
// R3.1: --backup creates a backup before replacing.
// R3.5: -f removes existing destination before creating.
func createLink(target, linkName string, opts linkOpts) int {
	// Check if destination already exists.
	if _, err := os.Lstat(linkName); err == nil {
		if !opts.force && !opts.backup {
			// R1.4: error when destination exists without -f or --backup.
			fmt.Fprintf(os.Stderr, "%s: failed to create %s link '%s': File exists\n",
				programName, linkType(opts.symbolic), linkName)
			return 1
		}
		if opts.backup {
			// R3.1: create backup by renaming the destination.
			if err := makeBackup(linkName, opts.backupMethod, opts.suffix); err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot backup '%s': %s\n",
					programName, linkName, err)
				return 1
			}
		} else {
			// R3.5: -f without backup removes existing destination.
			if err := os.Remove(linkName); err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n",
					programName, linkName, errMessage(err))
				return 1
			}
		}
	}

	if opts.symbolic {
		symlinkTarget := target
		// R2.4: compute relative path from the link's directory to the target.
		if opts.relative {
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
		// R3.4: verbose output for symlink.
		if opts.verbose {
			fmt.Printf("'%s' -> '%s'\n", linkName, symlinkTarget)
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
	// R3.4: verbose output for hard link.
	if opts.verbose {
		fmt.Printf("'%s' => '%s'\n", linkName, target)
	}
	return 0
}

// makeBackup creates a backup of the file at path using the specified method
// and suffix. Returns nil on success.
//
// R3.1: simple backups append the suffix.
// R3.3: numbered backups use .~N~ pattern.
// R3.3: existing method uses numbered if numbered backups exist, else simple.
func makeBackup(path, method, suffix string) error {
	switch method {
	case backupSimple:
		return os.Rename(path, path+suffix)
	case backupNumbered:
		return os.Rename(path, nextNumberedBackup(path))
	case backupExisting:
		if hasNumberedBackups(path) {
			return os.Rename(path, nextNumberedBackup(path))
		}
		return os.Rename(path, path+suffix)
	default:
		return os.Rename(path, path+suffix)
	}
}

// nextNumberedBackup returns the next available numbered backup path for the
// given file, using the .~N~ naming pattern. Starts at 1 and increments.
//
// R3.3: numbered backups use .~N~ pattern matching GNU ln behavior.
func nextNumberedBackup(path string) string {
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s.~%d~", path, n)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// hasNumberedBackups returns true if any .~N~ backup exists for the given path.
//
// R3.3: existing method checks for prior numbered backups.
func hasNumberedBackups(path string) bool {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	prefix := base + ".~"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, "~") {
			middle := name[len(prefix) : len(name)-1]
			if _, numErr := strconv.Atoi(middle); numErr == nil {
				return true
			}
		}
	}
	return false
}

// resolveBackupMethod maps a backup method string (or its alias) to a canonical
// method name. Returns an error for unrecognized methods.
//
// R3.3: "numbered"/"t", "existing"/"nil", "simple"/"never", "none"/"off".
func resolveBackupMethod(method string) (string, error) {
	switch method {
	case "none", "off":
		return backupNone, nil
	case "numbered", "t":
		return backupNumbered, nil
	case "existing", "nil":
		return backupExisting, nil
	case "simple", "never":
		return backupSimple, nil
	default:
		return "", fmt.Errorf("invalid argument '%s' for '--backup'", method)
	}
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

      --backup[=CONTROL]  make a backup of each existing destination file
  -b                      like --backup but does not accept an argument
  -f, --force             remove existing destination files
  -n, --no-dereference    treat LINK_NAME as a normal file if it is a
                            symbolic link to a directory
  -r, --relative          with -s, create relative symbolic links
  -s, --symbolic          make symbolic links instead of hard links
  -S, --suffix=SUFFIX     override the usual backup suffix
  -v, --verbose           print name of each linked file
      --help              display this help and exit
      --version           output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
//
// R1.4: --version prints version info to stdout and exits 0.
func printVersion() {
	fmt.Println("ln (go-unix-utils) 0.1")
}
