// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd039-env R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "env"

// exitNotFound is the exit code when COMMAND is not found.
const exitNotFound = 127

// exitCannotExec is the exit code when COMMAND is found but cannot be executed.
const exitCannotExec = 126

// exitInvalidOption is the exit code for invalid options.
const exitInvalidOption = 125

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// D3: Parse flags, NAME=VALUE assignments, and COMMAND separately.
	ignoreEnv := false
	nullTerminate := false
	var unsetNames []string
	var assignments []string
	var command []string

	i := 0
	for i < len(args) {
		arg := args[i]

		// D4: --help prints usage to stdout and exits 0.
		if arg == "--help" {
			printHelp()
			return
		}

		// D4: --version prints version and exits 0.
		if arg == "--version" {
			printVersion()
			return
		}

		// R2.1: -i / --ignore-environment starts with empty environment.
		if arg == "-i" || arg == "--ignore-environment" {
			ignoreEnv = true
			i++
			continue
		}

		// R3.1: -0 / --null terminates output lines with NUL instead of newline.
		if arg == "-0" || arg == "--null" {
			nullTerminate = true
			i++
			continue
		}

		// R2.2: -u NAME / --unset=NAME removes a variable from the environment.
		if arg == "-u" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'u'\n", programName)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
				os.Exit(exitInvalidOption)
			}
			unsetNames = append(unsetNames, args[i])
			i++
			continue
		}
		if name, ok := strings.CutPrefix(arg, "--unset="); ok {
			unsetNames = append(unsetNames, name)
			i++
			continue
		}

		// End-of-options marker.
		if arg == "--" {
			i++
			break
		}

		// R3.3: Reject unknown flags with appropriate error message and exit 125.
		if strings.HasPrefix(arg, "--") && arg != "--" {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(exitInvalidOption)
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			// Short option: report just the first invalid character.
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, arg[1])
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(exitInvalidOption)
		}

		// R2.3: NAME=VALUE assignment or start of COMMAND.
		// The first argument that does not contain '=' and is not a flag is
		// the start of COMMAND.
		if strings.Contains(arg, "=") {
			assignments = append(assignments, arg)
			i++
			continue
		}

		// First non-flag, non-assignment argument starts COMMAND.
		break
	}

	// Everything from index i onward is the COMMAND and its arguments.
	if i < len(args) {
		command = args[i:]
	}

	// D2: Build the environment.
	var env []string
	if ignoreEnv {
		// R2.1: start with empty environment.
		env = []string{}
	} else {
		env = os.Environ()
	}

	// R2.2: Remove unset variables from the environment.
	for _, name := range unsetNames {
		env = unsetEnvVar(env, name)
	}

	// R2.3: Apply NAME=VALUE assignments.
	for _, assignment := range assignments {
		env = setEnvVar(env, assignment)
	}

	// R1.1: No command — print environment and exit 0.
	if len(command) == 0 {
		// R3.1: Use NUL terminator when -0 / --null is set.
		terminator := "\n"
		if nullTerminate {
			terminator = "\x00"
		}
		for _, kv := range env {
			fmt.Print(kv + terminator)
		}
		return
	}

	// R3.1: -0/--null is incompatible with running a command.
	if nullTerminate {
		fmt.Fprintf(os.Stderr, "%s: cannot specify --null (-0) with command\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(exitInvalidOption)
	}

	// R1.2 / R2.1: Execute COMMAND with modified environment.
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// R1.3: exit with command's exit status.
			os.Exit(exitErr.ExitCode())
		}
		// R1.3: command not found → 127, cannot execute → 126.
		if isNotFound(err) {
			fmt.Fprintf(os.Stderr, "%s: '%s': No such file or directory\n", programName, command[0])
			os.Exit(exitNotFound)
		}
		// R1.3: Permission denied or other exec error → 126.
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", programName, command[0], execErr.Err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", programName, command[0], err.Error())
		}
		os.Exit(exitCannotExec)
	}
}

// unsetEnvVar removes all entries for the given variable name from the env slice.
//
// R2.2: -u NAME removes the variable from the environment.
func unsetEnvVar(env []string, name string) []string {
	prefix := name + "="
	result := env[:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, prefix) {
			result = append(result, kv)
		}
	}
	return result
}

// setEnvVar sets or overrides a NAME=VALUE entry in the env slice. If NAME
// already exists, it is replaced in place. Otherwise the entry is appended.
func setEnvVar(env []string, assignment string) []string {
	name, _, _ := strings.Cut(assignment, "=")
	prefix := name + "="
	for i, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			env[i] = assignment
			return env
		}
	}
	return append(env, assignment)
}

// isNotFound returns true when the error indicates the command binary was not
// found on the system.
func isNotFound(err error) bool {
	var e *exec.Error
	if errors.As(err, &e) {
		return os.IsNotExist(e.Err) || errors.Is(e.Err, exec.ErrNotFound)
	}
	return false
}

// printHelp writes usage information to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]
Set each NAME to VALUE in the environment and run COMMAND.

  -i, --ignore-environment  start with an empty environment
  -0, --null           end each output line with NUL, not newline
  -u, --unset=NAME     remove variable from the environment
      --help     display this help and exit
      --version  output version information and exit

A mere - implies -i.  If no COMMAND, print the resulting environment.
`)
}

// printVersion writes version information to stdout and exits 0.
func printVersion() {
	fmt.Println("env (go-unix-utils) 0.1")
}
