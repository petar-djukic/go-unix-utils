// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nice: run a command with modified scheduling priority.
// Implements srd094-nice R1.1-R1.4, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "nice"

const (
	// defaultAdjustment is the default niceness increment. R1.1.
	defaultAdjustment = 10
	exitInternal      = 125
	exitNotExec       = 126
	exitNotFound      = 127
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and executes the nice command.
// Returns the exit code for the process.
func run(args []string) int {
	adjustment, command, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return exitInternal
	}
	return execute(adjustment, command)
}

// parseArgs parses flags and positional arguments.
// Returns the adjustment value and the command with its arguments.
func parseArgs(args []string) (int, []string, error) {
	adjustment := defaultAdjustment
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-n" || arg == "--adjustment" {
			if i+1 >= len(args) {
				return 0, nil, fmt.Errorf(
					"option requires an argument -- 'n'")
			}
			val, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, nil, fmt.Errorf(
					"invalid adjustment '%s'", args[i+1])
			}
			adjustment = val
			i += 2
			continue
		}
		if matched, val, next, err := parseLongAdjustment(args, i); matched {
			if err != nil {
				return 0, nil, err
			}
			adjustment = val
			i = next
			continue
		}
		if matched, val, next, err := parseShortAdjustment(args, i); matched {
			if err != nil {
				return 0, nil, err
			}
			adjustment = val
			i = next
			continue
		}
		break
	}
	return adjustment, args[i:], nil
}

// parseLongAdjustment handles --adjustment=VALUE form.
// Returns (matched, value, nextIndex, error).
func parseLongAdjustment(args []string, i int) (bool, int, int, error) {
	arg := args[i]
	const prefix = "--adjustment="
	if len(arg) > len(prefix) && arg[:len(prefix)] == prefix {
		val, err := strconv.Atoi(arg[len(prefix):])
		if err != nil {
			return true, 0, 0, fmt.Errorf(
				"invalid adjustment '%s'", arg[len(prefix):])
		}
		return true, val, i + 1, nil
	}
	return false, 0, 0, nil
}

// parseShortAdjustment handles -nVALUE form (no space).
// Returns (matched, value, nextIndex, error).
func parseShortAdjustment(args []string, i int) (bool, int, int, error) {
	arg := args[i]
	if len(arg) > 2 && arg[0] == '-' && arg[1] == 'n' {
		val, err := strconv.Atoi(arg[2:])
		if err != nil {
			return true, 0, 0, fmt.Errorf(
				"invalid adjustment '%s'", arg[2:])
		}
		return true, val, i + 1, nil
	}
	return false, 0, 0, nil
}

// execute runs the command with the given niceness adjustment.
// R1.3: if command is empty, prints current nice value.
func execute(adjustment int, command []string) int {
	if len(command) == 0 {
		return printCurrentNice()
	}
	return runCommand(adjustment, command)
}

// printCurrentNice prints the current nice value and exits 0.
// R1.3: stub — returns 0.
func printCurrentNice() int {
	return 0
}

// runCommand applies the niceness adjustment and executes the command.
// R1.1, R1.4: stub — returns 0.
func runCommand(adjustment int, command []string) int {
	return 0
}

// Ensure imports are used. These variables exist only to satisfy
// the compiler for the contract stub and will be removed when
// the implementation is filled in.
var (
	_ = fmt.Sprintf
	_ = exec.Command
	_ = syscall.SIGTERM
	_ = strconv.Atoi
)
