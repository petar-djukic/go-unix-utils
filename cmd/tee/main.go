// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd017-tee R1.1-R1.5, R2.1-R2.3, R3.1-R3.4: cmd/tee reads
// stdin and writes to stdout and named output files simultaneously. Supports
// append mode (-a), SIGINT suppression (-i), and per-file error reporting.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU tee format.
const progName = "tee"

// destination represents a single output target for tee.
type destination struct {
	w      io.Writer
	name   string
	failed bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	appendMode := false
	ignoreInterrupts := false

	// Parse flags manually to match GNU tee flag handling.
	var files []string
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-a", "--append":
			appendMode = true
		case "-i", "--ignore-interrupts":
			ignoreInterrupts = true
		default:
			files = append(files, arg)
		}
	}

	// R2.2: ignore SIGINT when -i is specified.
	if ignoreInterrupts {
		signal.Ignore(os.Interrupt)
	}

	exitCode := 0

	// Build destination list: stdout first, then each file operand.
	dests := []destination{{w: os.Stdout, name: "stdout"}}
	var closers []*os.File

	for _, name := range files {
		// R1.4: "-" is an additional reference to stdout; already included.
		if name == "-" {
			continue
		}

		// R1.3, R2.1: create/truncate or append depending on -a flag.
		flag := os.O_WRONLY | os.O_CREATE
		if appendMode {
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
		f, err := os.OpenFile(name, flag, 0o666)
		if err != nil {
			// R3.2: report failed file and continue.
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		dests = append(dests, destination{w: f, name: name})
		closers = append(closers, f)
	}

	// R1.1, R1.5: read stdin and write to all destinations in order.
	buf := make([]byte, 8192)
	for {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 {
			for i := range dests {
				if dests[i].failed {
					continue
				}
				if _, werr := dests[i].w.Write(buf[:n]); werr != nil {
					// R3.3: report error but continue writing to other destinations.
					fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, dests[i].name, werr)
					dests[i].failed = true
					exitCode = 1
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, readErr)
				exitCode = 1
			}
			break
		}
	}

	// Close all opened files.
	for _, f := range closers {
		f.Close() // best-effort close
	}

	os.Exit(exitCode)
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
