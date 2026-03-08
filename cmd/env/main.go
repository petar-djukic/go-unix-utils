// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the env utility for running a command in a modified environment.
//
// Implements prd039-env: default behavior (R1), environment modification (R2),
// output formatting and exit codes (R3).
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	ignoreEnv := false
	nullTerm := false
	var unsetVars []string

	// Parse options manually. env has unusual parsing: NAME=VALUE args are positional
	// and flags must stop at the first non-option argument.
	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			break
		}

		if arg == "-i" || arg == "--ignore-environment" {
			ignoreEnv = true
			i++
			continue
		}

		if arg == "-0" || arg == "--null" {
			nullTerm = true
			i++
			continue
		}

		if arg == "-u" || arg == "--unset" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "env: option requires an argument -- 'u'\n")
				os.Exit(125)
			}
			i++
			unsetVars = append(unsetVars, args[i])
			i++
			continue
		}

		if strings.HasPrefix(arg, "--unset=") {
			unsetVars = append(unsetVars, arg[len("--unset="):])
			i++
			continue
		}

		if strings.HasPrefix(arg, "-u") && len(arg) > 2 {
			unsetVars = append(unsetVars, arg[2:])
			i++
			continue
		}

		if strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") && arg != "-" {
			fmt.Fprintf(os.Stderr, "env: invalid option -- '%s'\n", arg[1:])
			os.Exit(125)
		}

		// Not a flag: either NAME=VALUE or COMMAND.
		break
	}

	// Build the environment.
	var environ []string
	if !ignoreEnv {
		environ = os.Environ()
	}

	// Remove unset variables.
	for _, name := range unsetVars {
		environ = removeEnv(environ, name)
	}

	// Consume NAME=VALUE arguments.
	for i < len(args) && strings.Contains(args[i], "=") {
		environ = setEnv(environ, args[i])
		i++
	}

	// Remaining args are the command to execute.
	cmdArgs := args[i:]

	if len(cmdArgs) == 0 {
		// Print environment.
		w := bufio.NewWriter(os.Stdout)
		terminator := byte('\n')
		if nullTerm {
			terminator = 0
		}
		for _, env := range environ {
			w.WriteString(env)
			w.WriteByte(terminator)
		}
		w.Flush()
		return
	}

	// Execute command.
	binary, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "env: '%s': No such file or directory\n", cmdArgs[0])
		os.Exit(127)
	}

	// Use syscall.Exec to replace the process, matching GNU env behavior.
	err = syscall.Exec(binary, cmdArgs, environ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "env: '%s': %v\n", cmdArgs[0], err)
		os.Exit(126)
	}
}

func removeEnv(environ []string, name string) []string {
	prefix := name + "="
	result := environ[:0]
	for _, env := range environ {
		if !strings.HasPrefix(env, prefix) {
			result = append(result, env)
		}
	}
	return result
}

func setEnv(environ []string, pair string) []string {
	name, _, ok := strings.Cut(pair, "=")
	if !ok {
		return environ
	}
	prefix := name + "="
	for i, env := range environ {
		if strings.HasPrefix(env, prefix) {
			environ[i] = pair
			return environ
		}
	}
	return append(environ, pair)
}
