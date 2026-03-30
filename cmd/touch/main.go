// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/touch implements GNU touch: create files and update timestamps.
//
// Implements prd062-touch R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// touchOptions holds parsed flag state.
type touchOptions struct {
	noCreate bool // -c, --no-create: do not create files
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run parses flags and processes each file argument.
// R1.4: processes multiple files in order.
// R4.1: exits 0 on success, 1 on any error.
func run(args []string, stderr *os.File) int {
	opts, files := parseArgs(args)

	if len(files) == 0 {
		fmt.Fprintln(stderr, "touch: missing file operand")
		return 1
	}

	exitCode := 0
	now := time.Now()
	for _, f := range files {
		if err := touchFile(f, now, opts); err != nil {
			fmt.Fprintf(stderr, "touch: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (touchOptions, []string) {
	var opts touchOptions
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--no-create" {
			opts.noCreate = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			// Unknown long flag — treat as file
			files = append(files, arg)
			continue
		}
		if len(arg) >= 2 && arg[0] == '-' {
			parseShortFlags(&opts, arg[1:])
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// parseShortFlags processes short flag characters.
// R1.3: -c suppresses file creation.
func parseShortFlags(opts *touchOptions, chars string) {
	for _, ch := range chars {
		switch ch {
		case 'c':
			opts.noCreate = true
		}
	}
}

// touchFile updates timestamps or creates a single file.
// R1.1: updates atime and mtime to t.
// R1.2: creates the file if it does not exist.
// R1.3: skips creation when noCreate is set.
func touchFile(path string, t time.Time, opts touchOptions) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		if opts.noCreate {
			return nil
		}
		return createAndTouch(path, t)
	}
	if err != nil {
		return err
	}
	return os.Chtimes(path, t, t)
}

// createAndTouch creates an empty file and sets its timestamps.
func createAndTouch(path string, t time.Time) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	f.Close() // best-effort; error on Chtimes is more important
	return os.Chtimes(path, t, t)
}
