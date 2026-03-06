// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the tee utility.
// Implements prd017-tee: copy stdin to stdout and zero or more files.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	programName = "tee"
	versionStr  = "1.0.0"
	bufSize     = 32 * 1024
)

// outputErrorMode controls error handling behavior for write errors.
type outputErrorMode int

const (
	// R4: default — diagnose errors to non-pipe outputs, continue, exit 1 at end.
	outputErrorDefault outputErrorMode = iota
	// R4: warn — diagnose all errors, continue.
	outputErrorWarn
	// R4: warn-nopipe — diagnose non-pipe errors, continue.
	outputErrorWarnNoPipe
	// R4: exit — exit immediately on any error.
	outputErrorExit
	// R4: exit-nopipe — exit immediately on non-pipe error.
	outputErrorExitNoPipe
)

// output tracks a single write destination and its error state.
type output struct {
	writer io.Writer
	name   string
	isPipe bool
	failed bool
}

func main() {
	// D1: SIGPIPE handling per ARCHITECTURE.yaml shared protocol.
	installSIGPIPEHandler()

	appendMode, ignoreInterrupts, errMode, files := parseArgs(os.Args[1:])

	// R3: ignore SIGINT when -i is set.
	if ignoreInterrupts {
		signal.Ignore(syscall.SIGINT)
	}

	os.Exit(run(files, appendMode, errMode))
}

// installSIGPIPEHandler sets up SIGPIPE handling so tee exits cleanly
// when a downstream consumer closes its stdin.
func installSIGPIPEHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGPIPE)
	go func() {
		<-ch
		os.Exit(0)
	}()
}

func parseArgs(args []string) (appendMode, ignoreInterrupts bool, errMode outputErrorMode, files []string) {
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		switch {
		case arg == "--append":
			appendMode = true
		case arg == "--ignore-interrupts":
			ignoreInterrupts = true
		case arg == "--version":
			fmt.Printf("%s (go-unix-utils) %s\n", programName, versionStr)
			os.Exit(0)
		case arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--output-error":
			// R4: --output-error without =MODE defaults to "warn".
			errMode = outputErrorWarn
		case strings.HasPrefix(arg, "--output-error="):
			errMode = parseOutputErrorMode(arg[len("--output-error="):])
		case len(arg) > 1 && arg[0] == '-':
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					appendMode = true
				case 'i':
					ignoreInterrupts = true
				case 'p':
					// -p is shorthand for --output-error=warn-nopipe.
					errMode = outputErrorWarnNoPipe
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\nTry '%s --help' for more information.\n", programName, ch, programName)
					os.Exit(1)
				}
			}
		default:
			files = append(files, arg)
		}
	}
	return
}

func parseOutputErrorMode(mode string) outputErrorMode {
	switch mode {
	case "warn":
		return outputErrorWarn
	case "warn-nopipe":
		return outputErrorWarnNoPipe
	case "exit":
		return outputErrorExit
	case "exit-nopipe":
		return outputErrorExitNoPipe
	default:
		fmt.Fprintf(os.Stderr, "%s: invalid --output-error mode: '%s'\n", programName, mode)
		os.Exit(1)
		return outputErrorDefault
	}
}

func printUsage() {
	const usage = `Usage: %s [OPTION]... [FILE]...
Copy standard input to each FILE, and also to standard output.

  -a, --append             append to the given FILEs, do not overwrite
  -i, --ignore-interrupts  ignore interrupt signals
  -p                       operate in a more appropriate MODE with pipes
      --output-error[=MODE]  set behavior on write error
      --help               display this help and exit
      --version            output version information and exit
`
	_, _ = fmt.Fprintf(os.Stdout, usage, programName)
}

// run opens output files, reads stdin in chunks, and fans out writes.
// R1, R2, R3, R4: core tee logic.
func run(files []string, appendMode bool, errMode outputErrorMode) int {
	// D3: file open flags.
	openFlags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		openFlags |= os.O_APPEND
	} else {
		openFlags |= os.O_TRUNC
	}

	outputs := []output{
		{writer: os.Stdout, name: "standard output", isPipe: fdIsPipe(os.Stdout)},
	}

	var openFiles []*os.File
	hadError := false

	for _, path := range files {
		if path == "-" {
			// R1.4: '-' refers to stdout.
			outputs = append(outputs, output{writer: os.Stdout, name: "standard output", isPipe: true})
			continue
		}
		f, err := os.OpenFile(path, openFlags, 0o666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, formatPathError(err))
			hadError = true
			continue
		}
		openFiles = append(openFiles, f)
		outputs = append(outputs, output{writer: f, name: path, isPipe: fdIsPipe(f)})
	}

	// D4: read stdin in a loop using a fixed buffer.
	buf := make([]byte, bufSize)
	for {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			for i := range outputs {
				if outputs[i].failed {
					continue
				}
				_, writeErr := outputs[i].writer.Write(chunk)
				if writeErr != nil {
					o := &outputs[i]
					o.failed = true
					if shouldDiagnose(errMode, o.isPipe) {
						fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, o.name, writeErr.Error())
					}
					if shouldExitImmediately(errMode, o.isPipe) {
						closeFiles(openFiles)
						return 1
					}
					hadError = true
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				fmt.Fprintf(os.Stderr, "%s: read error: %s\n", programName, readErr.Error())
				hadError = true
			}
			break
		}
	}

	closeFiles(openFiles)

	if hadError {
		return 1
	}
	return 0
}

// shouldDiagnose returns true if a write error should be reported to stderr.
func shouldDiagnose(mode outputErrorMode, isPipe bool) bool {
	switch mode {
	case outputErrorDefault, outputErrorWarnNoPipe, outputErrorExitNoPipe:
		return !isPipe
	case outputErrorWarn, outputErrorExit:
		return true
	}
	return false
}

// shouldExitImmediately returns true if a write error should cause immediate exit.
func shouldExitImmediately(mode outputErrorMode, isPipe bool) bool {
	switch mode {
	case outputErrorExit:
		return true
	case outputErrorExitNoPipe:
		return !isPipe
	}
	return false
}

// fdIsPipe returns true if the file descriptor refers to a FIFO/pipe.
func fdIsPipe(f *os.File) bool {
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &stat); err != nil {
		return false
	}
	return stat.Mode&syscall.S_IFMT == syscall.S_IFIFO
}

// formatPathError formats an os.PathError for tee-style diagnostics.
func formatPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("%s: %s", pe.Path, pe.Err.Error())
	}
	return err.Error()
}

func closeFiles(files []*os.File) {
	for _, f := range files {
		_ = f.Close() // best-effort close, error ignored
	}
}
