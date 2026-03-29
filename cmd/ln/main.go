// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ln implements GNU ln: create links between files.
//
// Implements prd037-ln R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "ln"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run executes the ln logic and returns the exit code.
func run(args []string, stderr *os.File) int {
	if len(args) == 0 {
		printError(stderr, "missing file operand")
		printTryHelp(stderr)
		return 1
	}

	if len(args) == 1 {
		// R1.1: ln TARGET — create link in current directory with same basename.
		return linkSingle(args[0], filepath.Base(args[0]), stderr)
	}

	dest := args[len(args)-1]
	targets := args[:len(args)-1]

	// Check if destination is an existing directory.
	if isDir(dest) {
		// R1.2: ln TARGET... DIRECTORY
		return linkIntoDir(targets, dest, stderr)
	}

	if len(targets) > 1 {
		printError(stderr, fmt.Sprintf("target '%s' is not a directory", dest))
		return 1
	}

	// R1.1: ln TARGET LINK_NAME
	return linkSingle(targets[0], dest, stderr)
}

// linkSingle creates a single hard link from target to linkName.
// R1.1: hard link creation.
// R1.3: rejects directories.
// R1.4: rejects existing destinations.
func linkSingle(target, linkName string, stderr *os.File) int {
	if err := validateHardLinkTarget(target); err != nil {
		printError(stderr, fmt.Sprintf("hard link not allowed for directory '%s'", target))
		return 1
	}

	if err := os.Link(target, linkName); err != nil {
		printLinkError(stderr, linkName, err)
		return 1
	}

	return 0
}

// linkIntoDir creates hard links for each target inside dir.
// R1.2: multiple targets into a directory.
func linkIntoDir(targets []string, dir string, stderr *os.File) int {
	exitCode := 0
	for _, target := range targets {
		linkName := filepath.Join(dir, filepath.Base(target))
		if linkSingle(target, linkName, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// validateHardLinkTarget returns an error if target is a directory.
// R1.3: hard links to directories are not allowed.
func validateHardLinkTarget(target string) error {
	info, err := os.Stat(target)
	if err != nil {
		return nil // Let os.Link report the actual error.
	}
	if info.IsDir() {
		return fmt.Errorf("directory")
	}
	return nil
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// printError prints a formatted error to stderr.
func printError(stderr *os.File, msg string) {
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}

// printTryHelp prints the "try help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// printLinkError prints a link failure error to stderr.
// R1.4: reports existing destination errors.
func printLinkError(stderr *os.File, linkName string, err error) {
	fmt.Fprintf(stderr, "%s: failed to create hard link '%s': %s\n", progName, linkName, err) //nolint:errcheck
}
