// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd039-env R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3.
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
	nullTerm := false
	var unsets []string

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
		if arg == "-u" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "env: option requires an argument -- 'u'\n")
				fmt.Fprintf(os.Stderr, "Try 'env --help' for more information.\n")
				os.Exit(125)
			}
			unsets = append(unsets, args[i])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--unset=") {
			unsets = append(unsets, arg[len("--unset="):])
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "env: unrecognized option '%s'\n", arg)
			fmt.Fprintf(os.Stderr, "Try 'env --help' for more information.\n")
			os.Exit(125)
		}
		consumed, badChar, ok := parseShortFlags(arg, &ignoreEnv, &nullTerm)
		if !ok {
			fmt.Fprintf(os.Stderr, "env: invalid option -- '%c'\n", badChar)
			fmt.Fprintf(os.Stderr, "Try 'env --help' for more information.\n")
			os.Exit(125)
		}
		if consumed != "" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "env: option requires an argument -- 'u'\n")
				fmt.Fprintf(os.Stderr, "Try 'env --help' for more information.\n")
				os.Exit(125)
			}
			unsets = append(unsets, args[i])
		}
		i++
	}

	env := buildEnv(ignoreEnv, unsets, args[i:])
	i += countAssignments(args[i:])

	if i >= len(args) {
		printEnv(env, nullTerm)
		return
	}

	execCommand(args[i:], env)
}

func parseShortFlags(arg string, ignoreEnv *bool, nullTerm *bool) (consumed string, badChar rune, ok bool) {
	for _, ch := range arg[1:] {
		switch ch {
		case 'i':
			*ignoreEnv = true
		case '0':
			*nullTerm = true
		case 'u':
			return "u", 0, true
		default:
			return "", ch, false
		}
	}
	return "", 0, true
}

func buildEnv(ignoreEnv bool, unsets []string, rest []string) []string {
	var env []string
	if !ignoreEnv {
		env = os.Environ()
	}
	for _, name := range unsets {
		env = removeVar(env, name)
	}
	for _, arg := range rest {
		if !strings.Contains(arg, "=") {
			break
		}
		env = append(env, arg)
	}
	return env
}

func removeVar(env []string, name string) []string {
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

func printEnv(env []string, nullTerm bool) {
	for _, e := range env {
		if nullTerm {
			fmt.Print(e + "\x00")
		} else {
			fmt.Println(e)
		}
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
