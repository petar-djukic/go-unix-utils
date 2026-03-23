// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd017-tee: Read Stdin and Write to Stdout and Files.
// Covers R1.1-R1.3 (core copy, passthrough, file creation/truncation),
// R2.1 (append mode), R2.2 (SIGPIPE handling), R2.3 (stdin-only mode).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	// R2.2: SIGPIPE handling per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg, files))
}

// config holds parsed flag state.
type config struct {
	appendMode bool
}

// run reads stdin and writes to stdout and all named files.
// R1.1: write to stdout and all files simultaneously.
// R2.3: when no files given, copies stdin to stdout only.
func run(cfg config, files []string) int {
	writers, closers, exitCode := openFiles(cfg, files)
	defer closeAll(closers)

	writers = append([]io.Writer{os.Stdout}, writers...)
	mw := io.MultiWriter(writers...)

	if _, err := io.Copy(mw, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "tee: %v\n", err)
		return 1
	}

	return exitCode
}

// openFiles opens all named files and returns writers and closers.
// R1.3: creates files that do not exist; truncates unless append mode.
func openFiles(cfg config, files []string) ([]io.Writer, []io.Closer, int) {
	var writers []io.Writer
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
			fmt.Fprintf(os.Stderr, "tee: %v\n", err)
			exitCode = 1
			continue
		}
		writers = append(writers, f)
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

  -a, --append       append to the given FILEs, do not overwrite

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
