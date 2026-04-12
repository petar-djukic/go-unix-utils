// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/env: run a command in a modified environment.
// Implements srd039-env R1.1, R1.2, R1.3, R2.1, R2.2, R3.1.
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
  -0, --null                end each output line with NUL, not newline
  -u, --unset=NAME          remove variable from the environment
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

// envOptions holds parsed flag state for env.
type envOptions struct {
	ignoreEnv  bool
	nullTerm   bool
	unsetNames []string
	pairs      []string
	cmdArgs    []string
}

// run parses arguments, modifies the environment, and either prints it or
// executes the command. Returns the exit code.
func run(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return exitCodeInvalidOption
	}

	env := buildEnv(opts)

	if len(opts.cmdArgs) == 0 {
		return printEnv(env, opts.nullTerm)
	}
	return executeCommand(opts.cmdArgs, env)
}

// buildEnv constructs the environment slice.
// R2.1: when ignoreEnv is true, start from empty; otherwise inherit.
// R2.2: unset names are removed before NAME=VALUE pairs are applied.
func buildEnv(opts envOptions) []string {
	var env []string
	if !opts.ignoreEnv {
		env = os.Environ()
	}
	env = applyUnsets(env, opts.unsetNames)
	return applyPairs(env, opts.pairs)
}

// applyUnsets removes named variables from the environment. R2.2.
func applyUnsets(env []string, names []string) []string {
	for _, name := range names {
		env = removeEnvVar(env, name)
	}
	return env
}

// removeEnvVar removes a variable by name from the environment slice.
func removeEnvVar(env []string, name string) []string {
	prefix := name + "="
	n := 0
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			env[n] = e
			n++
		}
	}
	return env[:n]
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

// printEnv writes each environment variable to stdout. R1.1, R3.1.
func printEnv(env []string, nullTerm bool) int {
	sep := "\n"
	if nullTerm {
		sep = "\x00"
	}
	for _, e := range env {
		fmt.Print(e + sep)
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
// R2.2: -u NAME / --unset=NAME removes variables.
// R3.1: -0 / --null enables NUL-terminated output.
func parseArgs(args []string) (envOptions, error) {
	var opts envOptions
	i := 0
	for i < len(args) {
		arg := args[i]
		done, advance, err := parseFlag(arg, args, i, &opts)
		if err != nil {
			return envOptions{}, err
		}
		if done {
			i += advance
			break
		}
		if advance > 0 {
			i += advance
			continue
		}
		break
	}

	// Remaining args: NAME=VALUE pairs followed by COMMAND [ARG ...]
	for i < len(args) {
		if strings.Contains(args[i], "=") {
			opts.pairs = append(opts.pairs, args[i])
			i++
			continue
		}
		break
	}

	opts.cmdArgs = args[i:]
	return opts, nil
}

// parseFlag handles a single flag argument. Returns (done, advance, err)
// where done=true means stop flag parsing, advance is how many args consumed.
func parseFlag(arg string, args []string, i int, opts *envOptions) (bool, int, error) {
	if arg == "--help" {
		fmt.Print(helpText)
		os.Exit(0)
	}
	if arg == "--version" {
		fmt.Println(versionText)
		os.Exit(0)
	}
	if arg == "-i" || arg == "--ignore-environment" || arg == "-" {
		opts.ignoreEnv = true
		return false, 1, nil
	}
	if arg == "-0" || arg == "--null" {
		opts.nullTerm = true
		return false, 1, nil
	}
	if arg == "--" {
		return true, 1, nil
	}
	// R2.2: --unset=NAME form.
	if strings.HasPrefix(arg, "--unset=") {
		opts.unsetNames = append(opts.unsetNames, arg[len("--unset="):])
		return false, 1, nil
	}
	// R2.2: -u NAME (separate argument) or -uNAME (attached).
	if arg == "-u" {
		if i+1 >= len(args) {
			return false, 0, fmt.Errorf("option requires an argument -- 'u'")
		}
		opts.unsetNames = append(opts.unsetNames, args[i+1])
		return false, 2, nil
	}
	if strings.HasPrefix(arg, "-u") {
		opts.unsetNames = append(opts.unsetNames, arg[2:])
		return false, 1, nil
	}
	if strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") {
		return false, 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return true, 0, nil
}
