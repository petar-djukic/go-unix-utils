// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mkdir implements GNU mkdir: create directories.
//
// Implements prd034-mkdir R1.1, R1.2, R1.3, R1.4.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R1.4: install SIGPIPE handler for pipeline compatibility.
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run creates directories specified as positional arguments.
// R1.2: exits 0 on success, 1 on any failure.
// R1.3: reports errors per directory without aborting remaining arguments.
func run(args []string, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "mkdir: missing operand")                   //nolint:errcheck
		fmt.Fprintln(stderr, "Try 'mkdir --help' for more information.") //nolint:errcheck
		return 1
	}

	exitCode := 0
	for _, dir := range args {
		if err := os.Mkdir(dir, 0o777); err != nil {
			reportError(stderr, dir, err)
			exitCode = 1
		}
	}
	return exitCode
}

// reportError writes a mkdir error to stderr in GNU format.
// D3: format is "mkdir: cannot create directory 'NAME': reason".
func reportError(stderr *os.File, dir string, err error) {
	reason := err.Error()
	var pe *os.PathError
	if errors.As(err, &pe) {
		reason = pe.Err.Error()
	}
	fmt.Fprintf(stderr, "mkdir: cannot create directory '%s': %s\n", dir, reason) //nolint:errcheck
}
