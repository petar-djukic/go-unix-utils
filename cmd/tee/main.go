// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tee implements srd017-tee: read stdin and write to stdout and files.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	appendMode, files := parseFlags(os.Args[1:])
	writers, closers, exitCode := openOutputs(files, appendMode)
	defer closeAll(closers)

	if err := copyStdin(writers); err != nil {
		if !errors.Is(err, syscall.EPIPE) {
			fmt.Fprintf(os.Stderr, "tee: %v\n", err)
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func parseFlags(args []string) (bool, []string) {
	appendMode := false
	var files []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags || arg == "" || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		for _, c := range arg[1:] {
			switch c {
			case 'a':
				appendMode = true
			case 'i':
				// R2.2: accepted, full implementation in future task
			default:
				fmt.Fprintf(os.Stderr, "tee: invalid option -- '%c'\n", c)
				os.Exit(1)
			}
		}
	}
	return appendMode, files
}

// R1.1, R1.4: build writers for stdout and each named file.
func openOutputs(files []string, appendMode bool) ([]io.Writer, []io.Closer, int) {
	writers := []io.Writer{os.Stdout}
	var closers []io.Closer
	exitCode := 0
	for _, name := range files {
		if name == "-" {
			continue
		}
		f, err := openFile(name, appendMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tee: %s: %v\n", name, unwrapErr(err))
			exitCode = 1
			continue
		}
		writers = append(writers, f)
		closers = append(closers, f)
	}
	return writers, closers, exitCode
}

// R1.3: create or truncate; append if -a.
func openFile(name string, appendMode bool) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	return os.OpenFile(name, flags, 0o666)
}

// R1.1, R1.5: copy stdin to all writers in order received.
func copyStdin(writers []io.Writer) error {
	mw := io.MultiWriter(writers...)
	_, err := io.Copy(mw, os.Stdin)
	return err
}

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		c.Close()
	}
}

func unwrapErr(err error) error {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return pe.Err
	}
	return err
}
