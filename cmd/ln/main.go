// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd037-ln: Create Links Between Files.
// Covers R1.1-R1.4 (hard link creation, multi-target, error handling),
// R2.1-R2.2 (symbolic link creation via -s/--symbolic).
package main

import (
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
	symbolic bool
}

// run dispatches link creation and returns the exit code.
func run(cfg config, args []string) int {
	if len(args) == 1 {
		return linkToCurrentDir(cfg, args[0])
	}
	dest := args[len(args)-1]
	sources := args[:len(args)-1]

	if isDirectory(dest) {
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

// createLink creates a single hard or symbolic link.
// R1.1: hard link from target to linkName.
// R1.3: error on hard link to directory.
// R1.4: error when linkName already exists.
// R2.1: -s creates symbolic link.
// R2.2: symbolic links to directories are allowed.
func createLink(cfg config, target, linkName string) int {
	if cfg.symbolic {
		return createSymlink(target, linkName)
	}
	return createHardlink(target, linkName)
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

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// isDirectory returns true if path is an existing directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// parseArgs processes flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early termination.
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
		case arg == "-s" || arg == "--symbolic":
			cfg.symbolic = true
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

// parseShortFlags handles combined single-char flags like -s.
// Returns -1 to continue, >= 0 for early exit.
func parseShortFlags(arg string, cfg *config) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 's':
			cfg.symbolic = true
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

  -s, --symbolic  make symbolic links instead of hard links

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
