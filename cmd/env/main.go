// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/env: run a command in a modified environment.
// Implements srd039-env R1.1, R1.2, R1.3, R2.1.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "env"

const versionText = progName + " (go-unix-utils)"

const helpText = `Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]
Set each NAME to VALUE in the environment and run COMMAND.

  -i, --ignore-environment  start with an empty environment
      --help                display this help and exit
      --version             output version information and exit

A mere - implies -i.  If no COMMAND, print the resulting environment.
`

// exitCodeNotFound is returned when COMMAND is not found. R1.3.
const exitCodeNotFound = 127

// exitCodeNotExecutable is returned when COMMAND cannot be executed. R1.3.
const exitCodeNotExecutable = 126

// exitCodeInvalidOption is returned for invalid options. R3.3.
const exitCodeInvalidOption = 125

func main() {
	sys.InstallSIGPIPEHandler()

	code := run(os.Args[1:])
	os.Exit(code)
}

// run parses arguments, modifies the environment, and either prints it or
// executes the command. Returns the exit code.
func run(args []string) int {
	ignoreEnv, envPairs, cmdArgs, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return exitCodeInvalidOption
	}

	env := buildEnv(ignoreEnv, envPairs)

	if len(cmdArgs) == 0 {
		return printEnv(env)
	}
	return executeCommand(cmdArgs, env)
}

// buildEnv constructs the environment slice.
// R2.1: when ignoreEnv is true, start from empty; otherwise inherit.
func buildEnv(ignoreEnv bool, pairs []string) []string {
	var env []string
	if !ignoreEnv {
		env = os.Environ()
	}
	return applyPairs(env, pairs)
}

// applyPairs sets or overrides NAME=VALUE entries in the environment.
func applyPairs(env []string, pairs []string) []string {
	for _, pair := range pairs {
		name := pair[:strings.Index(pair, "=")]
		env = setEnvVar(env, name, pair)
	}
	return env
}

// setEnvVar replaces an existing variable or appends a new one.
func setEnvVar(env []string, name, pair string) []string {
	prefix := name + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = pair
			return env
		}
	}
	return append(env, pair)
}

// printEnv writes each environment variable to stdout. R1.1.
func printEnv(env []string) int {
	for _, e := range env {
		fmt.Println(e)
	}
	return 0
}

// executeCommand runs the specified command with the given environment.
// R1.2: execute COMMAND with modified environment.
// R1.3: exit 127 if not found, 126 if not executable.
func executeCommand(cmdArgs []string, env []string) int {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return 0
	}
	return extractExitCode(err, cmdArgs[0])
}

// extractExitCode determines the exit code from a command execution error.
func extractExitCode(err error, cmdName string) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	// R1.3: distinguish not-found from not-executable.
	if isNotFound(err) {
		fmt.Fprintf(os.Stderr, "%s: '%s': No such file or directory\n", progName, cmdName)
		return exitCodeNotFound
	}
	if isPermissionError(err) {
		fmt.Fprintf(os.Stderr, "%s: '%s': Permission denied\n", progName, cmdName)
		return exitCodeNotExecutable
	}
	fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", progName, cmdName, err)
	return exitCodeNotExecutable
}

// isNotFound checks if the error indicates the command was not found.
func isNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, syscall.ENOENT)
}

// isPermissionError checks if the error indicates a permission problem.
func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES)
}

// parseArgs separates flags, NAME=VALUE pairs, and command arguments.
// R1.1: NAME=VALUE pairs before COMMAND set environment variables.
// R2.1: -i / --ignore-environment starts with empty environment.
func parseArgs(args []string) (ignoreEnv bool, pairs []string, cmdArgs []string, err error) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println(versionText)
			os.Exit(0)
		}
		if arg == "-i" || arg == "--ignore-environment" || arg == "-" {
			ignoreEnv = true
			i++
			continue
		}
		if arg == "--" {
			i++
			break
		}
		if strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") {
			return false, nil, nil, fmt.Errorf("unrecognized option '%s'", arg)
		}
		break
	}

	// Remaining args: NAME=VALUE pairs followed by COMMAND [ARG ...]
	for i < len(args) {
		if strings.Contains(args[i], "=") {
			pairs = append(pairs, args[i])
			i++
			continue
		}
		break
	}

	cmdArgs = args[i:]
	return ignoreEnv, pairs, cmdArgs, nil
}
