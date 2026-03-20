// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd039-env R1.1, R1.2, R1.3, R2.1: env basic environment display,
// variable assignment, and command execution with -i/--ignore-environment flag.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "env"

// helpText is the usage message printed for --help.
const helpText = `Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]
Set each NAME to VALUE in the environment and run COMMAND.

  -i, --ignore-environment  start with an empty environment
      --help        display this help and exit
      --version     output version information and exit

A mere - implies -i.  If no COMMAND, print the resulting environment.
`

// versionText is the version message printed for --version.
const versionText = `env (go-unix-utils) 1.0
`

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	ignoreEnv, remaining := parseOptions(args)
	env := buildEnvironment(ignoreEnv)
	env, cmdStart := applyAssignments(remaining, env)
	if cmdStart >= len(remaining) {
		printEnvironment(env)
		return
	}
	executeCommand(remaining[cmdStart:], env)
}

// parseOptions processes option flags, returning whether -i was set and
// the remaining arguments after options. Handles --help and --version.
func parseOptions(args []string) (bool, []string) {
	ignoreEnv := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help":
			printAndExit(helpText)
		case "--version":
			printAndExit(versionText)
		case "-i", "--ignore-environment", "-":
			ignoreEnv = true
		case "--":
			return ignoreEnv, args[i+1:]
		default:
			return ignoreEnv, args[i:]
		}
	}
	return ignoreEnv, nil
}

// buildEnvironment returns the starting environment: empty slice for -i,
// or the current process environment otherwise. R2.1.
func buildEnvironment(ignoreEnv bool) []string {
	if ignoreEnv {
		return []string{}
	}
	return os.Environ()
}

// applyAssignments processes NAME=VALUE arguments, adding them to env.
// Returns the updated env and the index of the first non-assignment arg.
func applyAssignments(args []string, env []string) ([]string, int) {
	i := 0
	for i < len(args) && strings.Contains(args[i], "=") {
		env = setOrAppendEnv(env, args[i])
		i++
	}
	return env, i
}

// setOrAppendEnv sets or appends a NAME=VALUE pair in the env slice.
func setOrAppendEnv(env []string, assignment string) []string {
	idx := strings.IndexByte(assignment, '=')
	prefix := assignment[:idx+1]
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = assignment
			return env
		}
	}
	// Prepend new variables to match GNU env environ ordering.
	return append([]string{assignment}, env...)
}

// printEnvironment writes each env entry to stdout. R1.1.
func printEnvironment(env []string) {
	for _, e := range env {
		fmt.Println(e)
	}
}

// printAndExit writes text to stdout and exits 0 on success.
func printAndExit(text string) {
	if _, err := fmt.Fprint(os.Stdout, text); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// executeCommand runs the command with the modified environment.
// R1.2 (command execution) and R1.3 (exit code 127/126).
func executeCommand(cmdArgs []string, env []string) {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		handleExecError(cmdArgs[0], err)
	}
}

// handleExecError processes command execution errors, setting the
// appropriate exit code: 127 for not found, 126 for not executable.
func handleExecError(name string, err error) {
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", progName, name, extractOSError(err))
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		os.Exit(127)
	}
	os.Exit(126)
}

// extractOSError extracts the underlying OS error message from an exec error.
func extractOSError(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "No such file or directory"
	}
	return err.Error()
}
