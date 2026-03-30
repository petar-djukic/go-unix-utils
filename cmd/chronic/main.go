// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chronic implements moreutils chronic: run a command quietly unless it fails.
//
// Implements prd112-chronic R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "chronic"

// exitStderrTrigger is returned when -e triggers on stderr with exit 0.
const exitStderrTrigger = 2

// exitNotFound is returned when COMMAND cannot be found or executed (R2.2).
const exitNotFound = 100

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// config holds parsed flag state.
type config struct {
	stderr  bool
	verbose bool
	args    []string
}

// parseArgs extracts flags and the command from arguments.
func parseArgs(args []string) config {
	c := config{}
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-e":
			c.stderr = true
		case "-v":
			c.verbose = true
		default:
			c.args = args[i:]
			return c
		}
		i++
	}
	c.args = nil
	return c
}

// run parses arguments and executes the chronic logic. Returns exit code.
func run(args []string) int {
	cfg := parseArgs(args)
	if len(cfg.args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s COMMAND...\n", progName)
		return 255
	}
	return executeCommand(cfg)
}

// executeCommand runs the command, capturing output and deciding whether to show it.
// R1.1: suppress output on exit 0, show on non-zero.
func executeCommand(cfg config) int {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command(cfg.args[0], cfg.args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	// R2.2: command not found or cannot be executed → print error, exit 100.
	if isExecError(err) {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return exitNotFound
	}
	exitCode := exitCodeFromErr(err)

	return handleOutput(cfg, exitCode, &stdoutBuf, &stderrBuf)
}

// handleOutput decides whether to show output and returns the exit code.
func handleOutput(cfg config, exitCode int, stdout, stderr *bytes.Buffer) int {
	if exitCode != 0 {
		showOutput(cfg.verbose, exitCode, stdout, stderr)
		return exitCode
	}
	// R1.2: -e triggers display when command wrote to stderr, even on exit 0.
	if cfg.stderr && stderr.Len() > 0 {
		showOutput(cfg.verbose, exitCode, stdout, stderr)
		return exitStderrTrigger
	}
	return 0
}

// showOutput writes captured output to stdout/stderr.
// R1.3: -v adds STDOUT/STDERR/RETVAL headers around the output.
func showOutput(verbose bool, exitCode int, stdout, stderr *bytes.Buffer) {
	if verbose {
		fmt.Print("STDOUT:\n")
	}
	os.Stdout.Write(stdout.Bytes())
	if verbose {
		fmt.Print("\nSTDERR:\n")
	}
	os.Stdout.Sync() // best-effort flush before writing stderr
	os.Stderr.Write(stderr.Bytes())
	if verbose {
		fmt.Fprintf(os.Stdout, "\nRETVAL: %d\n", exitCode)
	}
}

// isExecError reports whether the error is an exec.Error (command not found).
func isExecError(err error) bool {
	_, ok := err.(*exec.Error)
	return ok
}

// exitCodeFromErr extracts the exit code from a command result.
// R2.1: propagates COMMAND exit status.
func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return exitNotFound
}
