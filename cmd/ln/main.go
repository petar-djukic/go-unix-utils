// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd037-ln R1.1–R1.4: hard link creation with single target,
// multi-target directory mode, directory rejection, and existing destination
// error handling matching GNU ln behavior.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "ln"

// helpText is the usage message printed for --help.
const helpText = `Usage: ln [OPTION]... TARGET LINK_NAME
  or:  ln [OPTION]... TARGET... DIRECTORY
Create hard links (by default) or symbolic links to files.

      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the version message printed for --version.
const versionText = `ln (go-unix-utils) 1.0
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "--help":
			printAndExit(helpText)
		case "--version":
			printAndExit(versionText)
		}
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\nTry '%s --help' for more information.\n", progName, progName)
		os.Exit(1)
	}

	if len(args) == 1 {
		fmt.Fprintf(os.Stderr, "%s: missing destination file operand after '%s'\nTry '%s --help' for more information.\n",
			progName, args[0], progName)
		os.Exit(1)
	}

	dest := args[len(args)-1]
	sources := args[:len(args)-1]

	if len(sources) > 1 {
		linkMultipleIntoDir(sources, dest)
	} else {
		linkSingle(sources[0], dest)
	}
}

// linkSingle creates a hard link from source to dest. If dest is an existing
// directory, the link is created inside it with the source's basename.
// Implements R1.1, R1.3, R1.4.
func linkSingle(source, dest string) {
	// R1.3: reject hard links to directories.
	if isDirectory(source) {
		fmt.Fprintf(os.Stderr, "%s: hard link not allowed for directory '%s'\n", progName, source)
		os.Exit(1)
	}

	// If dest is an existing directory, place link inside it.
	if isDirectory(dest) {
		dest = filepath.Join(dest, filepath.Base(source))
	}

	// R1.4: error when destination already exists.
	if _, err := os.Lstat(dest); err == nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create hard link '%s': File exists\n", progName, dest)
		os.Exit(1)
	}

	if err := os.Link(source, dest); err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to create hard link '%s' => '%s': %s\n",
			progName, dest, source, unwrapErr(err))
		os.Exit(1)
	}
}

// linkMultipleIntoDir creates hard links for each source in the target
// directory. Implements R1.2.
func linkMultipleIntoDir(sources []string, dir string) {
	if !isDirectory(dir) {
		fmt.Fprintf(os.Stderr, "%s: target '%s' is not a directory\n", progName, dir)
		os.Exit(1)
	}

	exitCode := 0
	for _, source := range sources {
		if err := linkIntoDir(source, dir); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// linkIntoDir creates a single hard link for source inside dir.
func linkIntoDir(source, dir string) error {
	// R1.3: reject hard links to directories.
	if isDirectory(source) {
		return fmt.Errorf("hard link not allowed for directory '%s'", source)
	}

	dest := filepath.Join(dir, filepath.Base(source))

	// R1.4: error when destination already exists.
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("failed to create hard link '%s': File exists", dest)
	}

	if err := os.Link(source, dest); err != nil {
		return fmt.Errorf("failed to create hard link '%s' => '%s': %s", dest, source, unwrapErr(err))
	}
	return nil
}

// isDirectory reports whether path is an existing directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// unwrapErr extracts the innermost error message, stripping os.PathError
// wrappers for cleaner output.
func unwrapErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	if le, ok := err.(*os.LinkError); ok {
		return le.Err.Error()
	}
	return err.Error()
}

// printAndExit writes text to stdout and exits 0 on success or 1 on write error.
func printAndExit(text string) {
	_, err := fmt.Fprint(os.Stdout, text)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
