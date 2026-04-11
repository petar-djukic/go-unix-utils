// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nice: run a command with modified scheduling priority.
// Implements srd094-nice R1.1-R1.4, R2.1-R2.3.
package main

import (
	"errors"
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

// optionError wraps errors that should show the "Try --help" hint,
// matching GNU nice behavior for getopt-style errors.
type optionError struct{ msg string }

func (e *optionError) Error() string { return e.msg }

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
		var oe *optionError
		if errors.As(err, &oe) {
			fmt.Fprintf(os.Stderr,
				"Try '%s --help' for more information.\n", progName)
		}
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
				return 0, nil, &optionError{
					msg: "option requires an argument -- 'n'"}
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

// printCurrentNice prints the current nice value and exits 0. R1.3.
func printCurrentNice() int {
	prio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: getpriority: %v\n", progName, err)
		return exitInternal
	}
	fmt.Println(prio)
	return 0
}

// runCommand applies the niceness adjustment and executes the command.
// R1.1, R1.2, R1.4, R2.1, R2.2.
func runCommand(adjustment int, command []string) int {
	if err := adjustPriority(adjustment); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot set niceness: %v\n",
			progName, err)
		// non-fatal per GNU nice: warn but continue
	}
	return execCommand(command)
}

// adjustPriority retrieves the current priority and sets the new one. R1.2, R1.3.
func adjustPriority(adjustment int) error {
	current, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return fmt.Errorf("getpriority: %w", err)
	}
	target := current + adjustment
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, target)
}

// formatLookPathError formats the error from exec.LookPath to match GNU nice
// output: "'cmd': No such file or directory" instead of Go's verbose format.
func formatLookPathError(name string, err error) string {
	var pathErr *exec.Error
	if errors.As(err, &pathErr) {
		return fmt.Sprintf("'%s': No such file or directory", name)
	}
	return fmt.Sprintf("%s: %v", name, err)
}

// execCommand replaces the process with the given command. R1.4, R2.1, R2.2.
func execCommand(command []string) int {
	binary, err := exec.LookPath(command[0])
	if err != nil {
		msg := formatLookPathError(command[0], err)
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
		return exitNotFound
	}
	err = syscall.Exec(binary, command, os.Environ())
	// syscall.Exec only returns on error
	fmt.Fprintf(os.Stderr, "%s: %s: %v\n",
		progName, command[0], err)
	return exitNotExec
}
