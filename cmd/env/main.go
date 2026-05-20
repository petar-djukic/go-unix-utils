// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	ignoreEnv := false

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
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		if !parseShortFlags(arg, &ignoreEnv) {
			fmt.Fprintf(os.Stderr, "env: invalid option -- '%s'\n", arg)
			os.Exit(125)
		}
		i++
	}

	env := buildEnv(ignoreEnv, args[i:])
	i += countAssignments(args[i:])

	if i >= len(args) {
		printEnv(env)
		return
	}

	execCommand(args[i:], env)
}

func parseShortFlags(arg string, ignoreEnv *bool) bool {
	for _, ch := range arg[1:] {
		if ch == 'i' {
			*ignoreEnv = true
		} else {
			return false
		}
	}
	return true
}

func buildEnv(ignoreEnv bool, rest []string) []string {
	var env []string
	if !ignoreEnv {
		env = os.Environ()
	}
	for _, arg := range rest {
		if !strings.Contains(arg, "=") {
			break
		}
		env = append(env, arg)
	}
	return env
}

func countAssignments(args []string) int {
	n := 0
	for _, arg := range args {
		if !strings.Contains(arg, "=") {
			break
		}
		n++
	}
	return n
}

func printEnv(env []string) {
	for _, e := range env {
		fmt.Println(e)
	}
}

func execCommand(args []string, env []string) {
	path, err := exec.LookPath(args[0])
	if err != nil {
		if isNotFound(err) {
			fmt.Fprintf(os.Stderr, "env: '%s': No such file or directory\n", args[0])
			os.Exit(127)
		}
		fmt.Fprintf(os.Stderr, "env: '%s': Permission denied\n", args[0])
		os.Exit(126)
	}

	cmd := exec.Command(path, args[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "env: '%s': %s\n", args[0], err)
		os.Exit(126)
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, syscall.ENOENT)
}
