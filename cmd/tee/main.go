// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd017-tee: Read Stdin and Write to Stdout and Files.
// Covers R1.1-R1.3 (core copy, passthrough, file creation/truncation),
// R2.1-R2.3 (append mode, SIGINT suppression), R3.1-R3.4 (error handling, exit codes).
//
// TODO: --output-error modes (warn, warn-nopipe, exit, exit-nopipe) are listed
// as non_goals in prd017. Skipped per E6 (non-goals enforcement).
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	// R2.2 (SIGPIPE): shared protocol handler.
	sys.InstallSIGPIPEHandler()

	cfg, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	// R2.2: ignore SIGINT when -i flag is set.
	if cfg.ignoreInterrupts {
		signal.Ignore(syscall.SIGINT)
	}

	os.Exit(run(cfg, files))
}

// config holds parsed flag state.
type config struct {
	appendMode       bool
	ignoreInterrupts bool
}

// namedWriter pairs a writer with a display name for error reporting.
type namedWriter struct {
	w    io.Writer
	name string
}

// resilientWriter writes to multiple destinations, continuing on failure.
// R3.3: continue writing to remaining files when one fails.
type resilientWriter struct {
	writers  []namedWriter
	failed   []bool
	exitCode int
}

// newResilientWriter creates a writer that fans out to all destinations.
func newResilientWriter(writers []namedWriter) *resilientWriter {
	return &resilientWriter{
		writers: writers,
		failed:  make([]bool, len(writers)),
	}
}

// Write sends p to every non-failed writer, reporting errors to stderr.
// R3.1: report write errors identifying the failed file.
// R3.2/R3.4: set exit code 1 on any write failure.
func (rw *resilientWriter) Write(p []byte) (int, error) {
	for i, nw := range rw.writers {
		if rw.failed[i] {
			continue
		}
		if _, err := nw.w.Write(p); err != nil {
			rw.failed[i] = true
			rw.exitCode = 1
			fmt.Fprintf(os.Stderr, "tee: %s: %v\n", nw.name, err)
		}
	}
	return len(p), nil
}

// run reads stdin and writes to stdout and all named files.
// R1.1: write to stdout and all files simultaneously.
// R2.3: when no files given, copies stdin to stdout only.
func run(cfg config, files []string) int {
	writers, closers, exitCode := openFiles(cfg, files)
	defer closeAll(closers)

	rw := newResilientWriter(writers)
	if _, err := io.Copy(rw, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "tee: %v\n", err)
		return 1
	}

	if rw.exitCode != 0 {
		return rw.exitCode
	}
	return exitCode
}

// openFiles opens all named files and returns named writers and closers.
// R1.3: creates files that do not exist; truncates unless append mode.
func openFiles(cfg config, files []string) ([]namedWriter, []io.Closer, int) {
	writers := []namedWriter{{w: os.Stdout, name: "standard output"}}
	var closers []io.Closer
	exitCode := 0

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if cfg.appendMode {
		// R2.1: append mode preserves existing content.
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	for _, name := range files {
		f, err := os.OpenFile(name, flags, 0o666)
		if err != nil {
			// R3.2: report failure and set exit code.
			fmt.Fprintf(os.Stderr, "tee: %v\n", err)
			exitCode = 1
			continue
		}
		writers = append(writers, namedWriter{w: f, name: name})
		closers = append(closers, f)
	}

	return writers, closers, exitCode
}

// closeAll closes all closers (best-effort cleanup, errors ignored).
func closeAll(closers []io.Closer) {
	for _, c := range closers {
		// best-effort cleanup, error ignored
		c.Close()
	}
}

// parseArgs processes flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (cfg config, files []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			files = append(files, args[i+1:]...)
			return
		case arg == "--help":
			return config{}, nil, printHelp()
		case arg == "--version":
			return config{}, nil, printVersion()
		case arg == "-a" || arg == "--append":
			cfg.appendMode = true
		case arg == "-i" || arg == "--ignore-interrupts":
			cfg.ignoreInterrupts = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			if !handleShortFlags(arg[1:], &cfg) {
				fmt.Fprintf(os.Stderr,
					"tee: unrecognized option '%s'\n", arg)
				return config{}, nil, 1
			}
		default:
			files = append(files, args[i:]...)
			return
		}
	}
	return
}

// handleShortFlags processes combined short flags. Returns false on error.
func handleShortFlags(flags string, cfg *config) bool {
	for _, ch := range flags {
		switch ch {
		case 'a':
			cfg.appendMode = true
		case 'i':
			cfg.ignoreInterrupts = true
		default:
			return false
		}
	}
	return true
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: tee [OPTION]... [FILE]...
Copy standard input to each FILE, and also to standard output.

  -a, --append              append to the given FILEs, do not overwrite
  -i, --ignore-interrupts   ignore interrupt signals

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
	_, err := fmt.Fprintf(os.Stdout, "tee (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
