// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd037-ln: Create Links Between Files.
// Covers R1.1-R1.4 (hard link creation, multi-target, error handling),
// R2.1-R2.4 (symbolic links, relative path computation),
// R3.1-R3.4 (force, no-dereference, interactive, verbose).
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, targets, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "ln: missing file operand")
		fmt.Fprintln(os.Stderr, "Try 'ln --help' for more information.")
		os.Exit(1)
	}

	os.Exit(run(cfg, targets))
}

// config holds parsed flag state.
type config struct {
	symbolic      bool
	force         bool
	interactive   bool
	noDereference bool
	verbose       bool
	relative      bool
}

// run dispatches link creation and returns the exit code.
func run(cfg config, args []string) int {
	if len(args) == 1 {
		return linkToCurrentDir(cfg, args[0])
	}
	dest := args[len(args)-1]
	sources := args[:len(args)-1]

	// R3.2: with -n, Lstat prevents following symlink-to-directory.
	if isDirectoryDest(dest, cfg.noDereference) {
		return linkIntoDir(cfg, sources, dest)
	}
	if len(sources) > 1 {
		fmt.Fprintf(os.Stderr,
			"ln: target '%s': Not a directory\n", dest)
		return 1
	}
	return createLink(cfg, sources[0], dest)
}

// linkToCurrentDir creates a link in the current directory with the
// basename of target. R1.2: single target without destination.
func linkToCurrentDir(cfg config, target string) int {
	linkName := filepath.Base(target)
	return createLink(cfg, target, linkName)
}

// linkIntoDir creates links for each source inside destDir.
// R1.2: multiple TARGETs in DIRECTORY.
func linkIntoDir(cfg config, sources []string, destDir string) int {
	exitCode := 0
	for _, src := range sources {
		linkName := filepath.Join(destDir, filepath.Base(src))
		if code := createLink(cfg, src, linkName); code != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// createLink orchestrates a single link: relative path, destination
// preparation, link creation, and verbose output.
func createLink(cfg config, target, linkName string) int {
	// R2.4: compute relative path when -r and -s are both given.
	target = applyRelative(cfg, target, linkName)
	if target == "" {
		return 1
	}
	code := prepareDestination(cfg, linkName)
	if code >= 0 {
		return code
	}
	return performLink(cfg, target, linkName)
}

// applyRelative computes a relative target path when -r and -s are set.
// R2.3: without -r, the TARGET string is stored as-is.
// R2.4: with -r, a relative path from link location to target is computed.
func applyRelative(cfg config, target, linkName string) string {
	if !cfg.relative || !cfg.symbolic {
		return target
	}
	rel, err := computeRelative(target, linkName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ln: %s: %s\n", target, err)
		return ""
	}
	return rel
}

// computeRelative returns the relative path from the link's parent
// directory to the target. R2.4.
func computeRelative(target, linkName string) (string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	linkDir := filepath.Dir(linkName)
	absLinkDir, err := filepath.Abs(linkDir)
	if err != nil {
		return "", err
	}
	return filepath.Rel(absLinkDir, absTarget)
}

// prepareDestination handles an existing destination file.
// Returns -1 to proceed, 1 for error or interactive decline.
// R3.1: -f removes existing. R3.3: -i prompts before removal.
func prepareDestination(cfg config, linkName string) int {
	if !destExists(linkName) {
		return -1
	}
	if cfg.interactive {
		if !promptReplace(linkName) {
			return 1
		}
		return removeDest(linkName)
	}
	if cfg.force {
		return removeDest(linkName)
	}
	return -1 // let the OS call report the conflict
}

// removeDest removes the destination file or symlink.
// Returns -1 on success, 1 on error.
func removeDest(linkName string) int {
	if err := os.Remove(linkName); err != nil {
		fmt.Fprintf(os.Stderr, "ln: cannot remove '%s': %s\n",
			linkName, unwrapErrMsg(err))
		return 1
	}
	return -1
}

// promptReplace asks the user on stderr whether to replace linkName.
// R3.3: reads one line from stdin; proceeds only if response starts
// with 'y' or 'Y'.
func promptReplace(linkName string) bool {
	fmt.Fprintf(os.Stderr, "ln: replace '%s'? ", linkName)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(line)
	return len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
}

// performLink creates the link and prints verbose output on success.
// R3.4: -v prints the name of each link created to stdout.
func performLink(cfg config, target, linkName string) int {
	var code int
	if cfg.symbolic {
		code = createSymlink(target, linkName)
	} else {
		code = createHardlink(target, linkName)
	}
	if code == 0 && cfg.verbose {
		printVerbose(cfg.symbolic, linkName, target)
	}
	return code
}

// printVerbose prints the link creation message to stdout.
// R3.4: format matches GNU ln verbose output.
func printVerbose(symbolic bool, linkName, target string) {
	arrow := "=>"
	if symbolic {
		arrow = "->"
	}
	fmt.Fprintf(os.Stdout, "'%s' %s '%s'\n", linkName, arrow, target)
}

// createSymlink creates a symbolic link.
// R2.1-R2.2: symbolic links, including to directories.
func createSymlink(target, linkName string) int {
	if err := os.Symlink(target, linkName); err != nil {
		printLinkError(err)
		return 1
	}
	return 0
}

// createHardlink creates a hard link.
// R1.3: rejects directories. Checks target accessibility first.
func createHardlink(target, linkName string) int {
	info, err := os.Stat(target)
	if err != nil {
		printAccessError(target, err)
		return 1
	}
	if info.IsDir() {
		fmt.Fprintf(os.Stderr,
			"ln: %s: hard link not allowed for directory\n", target)
		return 1
	}
	if err := os.Link(target, linkName); err != nil {
		printLinkError(err)
		return 1
	}
	return 0
}

// destExists returns true if something exists at path (using Lstat
// to detect symlinks without following them).
func destExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// isDirectoryDest checks if path is a directory.
// R3.2: with noDereference, uses Lstat so a symlink to a directory
// is not treated as a directory.
func isDirectoryDest(path string, noDereference bool) bool {
	var info os.FileInfo
	var err error
	if noDereference {
		info, err = os.Lstat(path)
	} else {
		info, err = os.Stat(path)
	}
	return err == nil && info.IsDir()
}

// printAccessError prints GNU-style error for target access failure.
func printAccessError(target string, err error) {
	reason := "No such file or directory"
	if pe, ok := err.(*os.PathError); ok {
		reason = capitalizeFirst(pe.Err.Error())
	}
	fmt.Fprintf(os.Stderr,
		"ln: failed to access '%s': %s\n", target, reason)
}

// printLinkError formats a link error in GNU style.
func printLinkError(err error) {
	if le, ok := err.(*os.LinkError); ok {
		fmt.Fprintf(os.Stderr,
			"ln: failed to create %s link '%s': %s\n",
			linkType(le.Op), le.New,
			capitalizeFirst(le.Err.Error()))
		return
	}
	if pe, ok := err.(*os.PathError); ok {
		fmt.Fprintf(os.Stderr, "ln: %s\n", pe.Error())
		return
	}
	fmt.Fprintf(os.Stderr, "ln: %s\n", err.Error())
}

// linkType returns "hard" or "symbolic" based on the syscall operation.
func linkType(op string) string {
	if op == "symlink" {
		return "symbolic"
	}
	return "hard"
}

// unwrapErrMsg extracts the inner error message from an os.PathError.
func unwrapErrMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return capitalizeFirst(pe.Err.Error())
	}
	return err.Error()
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseArgs processes flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early termination.
// R3.3: when -f and -i are both given, the last one on the command line wins.
func parseArgs(args []string) (cfg config, targets []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			targets = append(targets, args[i+1:]...)
			return
		case arg == "--help":
			return config{}, nil, printHelp()
		case arg == "--version":
			return config{}, nil, printVersion()
		case arg == "--symbolic":
			cfg.symbolic = true
		case arg == "--force":
			cfg.force = true
			cfg.interactive = false
		case arg == "--interactive":
			cfg.interactive = true
			cfg.force = false
		case arg == "--no-dereference":
			cfg.noDereference = true
		case arg == "--verbose":
			cfg.verbose = true
		case arg == "--relative":
			cfg.relative = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1 &&
			!strings.HasPrefix(arg, "--"):
			exit = parseShortFlags(arg, &cfg)
			if exit >= 0 {
				return cfg, nil, exit
			}
		default:
			targets = append(targets, args[i:]...)
			return
		}
	}
	return
}

// parseShortFlags handles combined single-char flags like -sfn.
// Returns -1 to continue, >= 0 for early exit.
// R3.3: -f and -i override each other; last one wins.
func parseShortFlags(arg string, cfg *config) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 's':
			cfg.symbolic = true
		case 'f':
			cfg.force = true
			cfg.interactive = false
		case 'i':
			cfg.interactive = true
			cfg.force = false
		case 'n':
			cfg.noDereference = true
		case 'v':
			cfg.verbose = true
		case 'r':
			cfg.relative = true
		default:
			fmt.Fprintf(os.Stderr,
				"ln: unrecognized option '-%c'\n", arg[j])
			return 1
		}
	}
	return -1
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: ln [OPTION]... TARGET [LINK_NAME]
  or:  ln [OPTION]... TARGET... DIRECTORY
Create links between files.

  -f, --force            remove existing destination files
  -i, --interactive      prompt before removing destinations
  -n, --no-dereference   treat LINK_NAME as a normal file if
                           it is a symbolic link to a directory
  -r, --relative         with -s, create relative symbolic links
  -s, --symbolic         make symbolic links instead of hard links
  -v, --verbose          print name of each linked file

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout,
		"ln (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
