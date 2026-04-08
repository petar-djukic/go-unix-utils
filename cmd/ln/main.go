// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ln: create links between files.
// Implements srd037 R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "ln"

// R1.1: main entry with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	exitCode := run(args)
	os.Exit(exitCode)
}

// run dispatches to the correct link form based on argument count.
// R1.1: two-arg form creates a hard link named args[1] pointing to args[0].
// R1.2: multi-arg form creates hard links in the last arg (directory).
// R1.4: single-arg form creates a hard link in cwd with basename of target.
func run(args []string) int {
	switch len(args) {
	case 1:
		return linkSingleArg(args[0])
	case 2:
		return linkTwoArgs(args[0], args[1])
	default:
		return linkMultiArgs(args)
	}
}

// linkSingleArg implements the single-argument form: ln TARGET.
// R1.4: creates a hard link in the current directory with the same basename.
func linkSingleArg(target string) int {
	linkName := filepath.Base(target)
	if err := createHardLink(target, linkName); err != nil {
		printError(err)
		return 1
	}
	return 0
}

// linkTwoArgs implements the two-argument form: ln TARGET LINK_NAME.
// R1.1: creates a hard link named linkName pointing to target.
// If linkName is an existing directory, creates the link inside it.
func linkTwoArgs(target, linkName string) int {
	if isDir(linkName) {
		linkName = filepath.Join(linkName, filepath.Base(target))
	}
	if err := createHardLink(target, linkName); err != nil {
		printError(err)
		return 1
	}
	return 0
}

// linkMultiArgs implements the multi-argument form: ln TARGET... DIRECTORY.
// R1.2: creates hard links in directory for each target.
func linkMultiArgs(args []string) int {
	dir := args[len(args)-1]
	targets := args[:len(args)-1]

	if !isDir(dir) {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n",
			programName, dir)
		return 1
	}

	exitCode := 0
	for _, target := range targets {
		linkName := filepath.Join(dir, filepath.Base(target))
		if err := createHardLink(target, linkName); err != nil {
			printError(err)
			exitCode = 1
		}
	}
	return exitCode
}

// createHardLink creates a hard link from target to linkName.
// R1.3: returns error when target is a directory (hard links to dirs not allowed).
// R1.4: returns error when linkName already exists.
func createHardLink(target, linkName string) error {
	if err := os.Link(target, linkName); err != nil {
		return formatLinkError(linkName, err)
	}
	return nil
}

// formatLinkError wraps a link error to match GNU ln output format.
func formatLinkError(linkName string, err error) error {
	return fmt.Errorf("failed to create hard link '%s': %s",
		linkName, unwrapOSError(err))
}

// unwrapOSError extracts the underlying message from an *os.PathError or
// *os.LinkError.
func unwrapOSError(err error) string {
	if le, ok := err.(*os.LinkError); ok {
		return le.Err.Error()
	}
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printError prints a formatted error to stderr in GNU ln style.
func printError(err error) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
