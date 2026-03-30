// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nice implements GNU nice: run a command with modified scheduling priority.
//
// Implements prd094-nice R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "nice"

const defaultAdjustment = 10

const exitInternal = 125
const exitCannotExec = 126
const exitNotFound = 127

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and dispatches to print or execute.
func run(args []string) int {
	adjustment, cmdArgs, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return exitInternal
	}
	if len(cmdArgs) == 0 {
		return printCurrentNice()
	}
	return adjustAndExec(adjustment, cmdArgs)
}

// parseArgs extracts the adjustment value and command arguments.
// R1.2: -n ADJUST or --adjustment=ADJUST sets the increment.
func parseArgs(args []string) (int, []string, error) {
	adjustment := defaultAdjustment
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		adv, adj, err := handleFlag(arg, args, i)
		if err != nil {
			return 0, nil, err
		}
		adjustment = adj
		i += adv
	}
	return adjustment, args[i:], nil
}

// handleFlag processes a single flag, returning advance count and adjustment.
func handleFlag(arg string, args []string, i int) (int, int, error) {
	switch {
	case arg == "-n" || arg == "--adjustment":
		return parseAdjustmentNext(args, i)
	case strings.HasPrefix(arg, "--adjustment="):
		return parseAdjustmentValue(arg[len("--adjustment="):])
	default:
		return 0, 0, fmt.Errorf("invalid option -- '%s'",
			strings.TrimLeft(arg, "-"))
	}
}

// parseAdjustmentNext parses -n ADJUST or --adjustment ADJUST.
func parseAdjustmentNext(args []string, i int) (int, int, error) {
	if i+1 >= len(args) {
		return 0, 0, fmt.Errorf("option requires an argument -- 'n'")
	}
	n, err := strconv.Atoi(args[i+1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid adjustment '%s'", args[i+1])
	}
	return 2, n, nil
}

// parseAdjustmentValue parses the value from --adjustment=ADJUST.
func parseAdjustmentValue(val string) (int, int, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid adjustment '%s'", val)
	}
	return 1, n, nil
}

// printCurrentNice prints the current scheduling priority to stdout.
// R1.3: when invoked with no COMMAND, print nice value and exit 0.
func printCurrentNice() int {
	prio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: getpriority: %s\n", progName, err)
		return exitInternal
	}
	fmt.Println(prio)
	return 0
}

// adjustAndExec adjusts the scheduling priority and runs the command.
// R1.1: adjusts priority by increment (default 10).
// R1.4: passes all remaining args to the command.
func adjustAndExec(adjustment int, cmdArgs []string) int {
	if err := adjustPriority(adjustment); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot set niceness: %s\n",
			progName, err)
		return exitInternal
	}
	return executeCommand(cmdArgs)
}

// adjustPriority adds the increment to the current process priority.
func adjustPriority(adjustment int) error {
	current, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return err
	}
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, current+adjustment)
}

// executeCommand runs the command and returns its exit code.
func executeCommand(cmdArgs []string) int {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return handleExecError(err, cmdArgs[0])
	}
	return 0
}

// handleExecError maps command execution errors to exit codes.
func handleExecError(err error, cmdName string) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", progName, cmdName, err)
	if isNotFound(err) {
		return exitNotFound
	}
	return exitCannotExec
}

// isNotFound checks if the error indicates the command was not found.
func isNotFound(err error) bool {
	return strings.Contains(err.Error(), exec.ErrNotFound.Error())
}
