// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tee implements srd017-tee: read stdin and write to stdout and files.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// R3.4: ignore SIGPIPE so writes return EPIPE instead of killing the process.
	signal.Ignore(syscall.SIGPIPE)

	appendMode, ignoreInterrupts, files := parseFlags(os.Args[1:])
	if ignoreInterrupts {
		signal.Ignore(os.Interrupt)
	}
	outputs, closers, exitCode := openOutputs(files, appendMode)
	defer closeAll(closers)

	if copyToAll(outputs) {
		exitCode = 1
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func parseFlags(args []string) (bool, bool, []string) {
	appendMode := false
	ignoreInterrupts := false
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
		if arg == "--append" {
			appendMode = true
			continue
		}
		if arg == "--ignore-interrupts" {
			ignoreInterrupts = true
			continue
		}
		for _, c := range arg[1:] {
			switch c {
			case 'a':
				appendMode = true
			case 'i':
				ignoreInterrupts = true
			default:
				fmt.Fprintf(os.Stderr, "tee: invalid option -- '%c'\n", c)
				os.Exit(1)
			}
		}
	}
	return appendMode, ignoreInterrupts, files
}

type namedWriter struct {
	w    io.Writer
	name string
	ok   bool
}

// R1.1, R1.4, R3.2: build writers for stdout and each named file.
func openOutputs(files []string, appendMode bool) ([]namedWriter, []io.Closer, int) {
	outputs := []namedWriter{{w: os.Stdout, name: "standard output", ok: true}}
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
		outputs = append(outputs, namedWriter{w: f, name: name, ok: true})
		closers = append(closers, f)
	}
	return outputs, closers, exitCode
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

// R1.5, R3.3: copy stdin to all outputs; skip failed writers.
func copyToAll(outputs []namedWriter) bool {
	buf := make([]byte, 32*1024)
	hadError := false
	for {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 && writeAll(outputs, buf[:n]) {
			hadError = true
		}
		if readErr != nil {
			break
		}
	}
	return hadError
}

// R3.2, R3.3, R3.4: write data to each active output; disable on error.
func writeAll(outputs []namedWriter, data []byte) bool {
	hadError := false
	for i := range outputs {
		if !outputs[i].ok {
			continue
		}
		if _, err := outputs[i].w.Write(data); err != nil {
			outputs[i].ok = false
			fmt.Fprintf(os.Stderr, "tee: %s: %v\n", outputs[i].name, unwrapErr(err))
			hadError = true
		}
	}
	return hadError
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
