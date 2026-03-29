// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tee implements GNU tee: read stdin and write to stdout and files.
//
// Implements prd017-tee R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
//
// TODO: --output-error modes (warn, warn-nopipe, exit, exit-nopipe) are listed
// in prd017 non_goals and must not be implemented per article E6.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: tee [OPTION]... [FILE]...
Copy standard input to each FILE, and also to standard output.

  -a, --append             append to the given FILEs, do not overwrite
  -i, --ignore-interrupts  ignore interrupt signals
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "tee (go-unix-utils) 0.1\n"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and executes tee logic.
func run(args []string, stdin io.Reader, stdout, stderr *os.File) int {
	opts, result := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck // best-effort
		return 0
	}

	// R2.2: ignore SIGINT when -i is set.
	if opts.ignoreInterrupts {
		signal.Ignore(syscall.SIGINT)
	}

	return copyToFiles(opts, stdin, stdout, stderr)
}

// parseResult signals how argument parsing concluded.
type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp             // --help requested
	parseVer              // --version requested
)

// options holds parsed command-line flags.
type options struct {
	appendMode      bool
	ignoreInterrupts bool
	files           []string
}

// parseArgs parses GNU-style tee arguments.
func parseArgs(args []string) (*options, parseResult) {
	opts := &options{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--help" {
			return opts, parseHelp
		}
		if arg == "--version" {
			return opts, parseVer
		}
		if arg == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			return opts, parseOK
		}
		if arg == "--append" {
			opts.appendMode = true
			i++
			continue
		}
		if arg == "--ignore-interrupts" {
			opts.ignoreInterrupts = true
			i++
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			if parseShortFlags(arg, opts) {
				i++
				continue
			}
		}
		// Not a flag — this and all remaining args are file names.
		opts.files = append(opts.files, args[i:]...)
		return opts, parseOK
	}
	return opts, parseOK
}

// parseShortFlags handles short flags like -a, -i, -ai.
// Returns true if the arg was handled as flags.
func parseShortFlags(arg string, opts *options) bool {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'a':
			opts.appendMode = true
		case 'i':
			opts.ignoreInterrupts = true
		default:
			return false
		}
	}
	return true
}

// copyToFiles reads stdin and writes to stdout and all output files.
// R1.5: writes output in the order received from stdin.
func copyToFiles(opts *options, stdin io.Reader, stdout, stderr *os.File) int {
	files, exitCode := openFiles(opts, stderr)
	defer closeFiles(files)

	writers := buildWriters(stdout, files)
	w := io.MultiWriter(writers...)

	buf := make([]byte, 32*1024)
	for {
		n, readErr := stdin.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				fmt.Fprintf(stderr, "tee: write error: %v\n", writeErr) //nolint:errcheck
				exitCode = 1
			}
		}
		if readErr != nil {
			break
		}
	}

	return exitCode
}

// openFiles opens all output files and returns them along with an exit code.
func openFiles(opts *options, stderr *os.File) ([]*os.File, int) {
	flag := os.O_WRONLY | os.O_CREATE
	if opts.appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	exitCode := 0
	files := make([]*os.File, 0, len(opts.files))
	for _, name := range opts.files {
		f, err := os.OpenFile(name, flag, 0o666)
		if err != nil {
			fmt.Fprintf(stderr, "tee: %s: %v\n", name, unwrapMsg(err)) //nolint:errcheck
			exitCode = 1
			continue
		}
		files = append(files, f)
	}
	return files, exitCode
}

// buildWriters creates the list of io.Writers: stdout plus all open files.
func buildWriters(stdout *os.File, files []*os.File) []io.Writer {
	writers := make([]io.Writer, 0, 1+len(files))
	writers = append(writers, stdout)
	for _, f := range files {
		writers = append(writers, f)
	}
	return writers
}

// closeFiles closes all open output files.
func closeFiles(files []*os.File) {
	for _, f := range files {
		f.Close() // best-effort close
	}
}

// unwrapMsg extracts the inner message from a *os.PathError.
func unwrapMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
