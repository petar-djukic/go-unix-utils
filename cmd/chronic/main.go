// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/chronic: run a command quietly unless it fails.
// Implements srd112-chronic R1.1-R1.3, R2.1-R2.3.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "chronic"

// exitStderrTriggered is the exit code when -e triggers on stderr content.
const exitStderrTriggered = 2

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// options holds parsed command-line flags.
type options struct {
	verbose bool // -v/--verbose: use labeled output format on display
	stderr  bool // -e/--stderr: trigger output on stderr content even if exit 0
}

// run parses flags, validates arguments, and executes the command.
// R1.1-R1.3: core chronic behavior.
func run(args []string) int {
	opts, cmdArgs, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		printUsage()
		return 1
	}
	if len(cmdArgs) == 0 {
		printUsage()
		return 1
	}
	return executeCommand(cmdArgs, opts)
}

// parseFlags extracts -v/--verbose and -e/--stderr flags from args.
// Returns parsed options, remaining command args, and error for unknown flags.
func parseFlags(args []string) (options, []string, error) {
	var opts options
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			return opts, args[i+1:], nil
		}
		if !strings.HasPrefix(arg, "-") {
			return opts, args[i:], nil
		}
		switch arg {
		case "-v", "--verbose":
			opts.verbose = true
		case "-e", "--stderr":
			opts.stderr = true
		default:
			name := strings.TrimLeft(arg, "-")
			return opts, nil, fmt.Errorf("Unknown option: %s", name)
		}
	}
	return opts, nil, nil
}

// printUsage writes the usage message to stderr.
func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: chronic [-e] [-v] command ...")
}

// executeCommand runs the command and handles output suppression.
// R1.1: suppress on success, replay on failure.
// R1.2: -e triggers replay when stderr has content, exits 2.
// R1.3: -v uses labeled format for output.
// R2.1: exit with child's exit code.
func executeCommand(cmdArgs []string, opts options) int {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := extractExitCode(err)

	stderrTriggered := opts.stderr && stderr.Len() > 0 && exitCode == 0
	if exitCode != 0 || stderrTriggered {
		replayOutput(stdout.Bytes(), stderr.Bytes(), exitCode, opts)
	}
	if stderrTriggered {
		return exitStderrTriggered
	}
	return exitCode
}

// replayOutput displays captured command output.
// In verbose mode, uses labeled STDOUT:/STDERR:/RETVAL: format to stdout,
// while writing captured stderr to actual stderr.
// In normal mode, replays raw stdout and stderr to their respective streams.
func replayOutput(stdout, stderr []byte, exitCode int, opts options) {
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "STDOUT:\n%s\nSTDERR:\n\nRETVAL: %d\n",
			stdout, exitCode)
		os.Stderr.Write(stderr)
		return
	}
	os.Stdout.Write(stdout)
	os.Stderr.Write(stderr)
}

// extractExitCode returns the child process exit code.
// R2.1: propagate child exit code.
// R2.2: return 100 when command cannot be found or executed.
func extractExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	// R2.2: command not found or not executable.
	fmt.Fprintf(os.Stderr, "%s: failed to run command: %v\n", progName, err)
	return 100
}
