// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nice implements GNU nice: run a program with modified scheduling priority.
// Implements prd094-nice R1.1-R1.4, R2.1-R2.3.
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

const (
	progName     = "nice"
	defaultAdj   = 10
	exitInternal = 125
	exitNoExec   = 126
	exitNotFound = 127
)

func main() {
	sys.InstallSIGPIPEHandler()
	adj, cmd, args, err := parseArgs(os.Args[1:])
	if err != nil {
		exitWithError(err.Error())
	}
	if cmd == "" {
		printNiceness()
		return
	}
	os.Exit(runCommand(adj, cmd, args))
}

// parseArgs extracts the adjustment value, command, and command arguments.
// R1.2: accepts -n N, --adjustment=N, --adjustment N, and legacy -N.
func parseArgs(args []string) (int, string, []string, error) {
	adj := defaultAdj
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			i++
			break
		}
		if !strings.HasPrefix(args[i], "-") || args[i] == "-" {
			break
		}
		consumed, newAdj, err := parseFlag(args[i:])
		if err != nil {
			return 0, "", nil, err
		}
		if consumed == 0 {
			break
		}
		adj = newAdj
		i += consumed
	}
	if i >= len(args) {
		return adj, "", nil, nil
	}
	return adj, args[i], args[i+1:], nil
}

// parseFlag parses one flag cluster starting at args[0].
// Returns consumed arg count, the parsed adjustment, and any error.
func parseFlag(args []string) (int, int, error) {
	arg := args[0]
	switch {
	case arg == "--help":
		printHelp()
		os.Exit(0)
	case arg == "--version":
		printVersion()
		os.Exit(0)
	case arg == "-n" || arg == "--adjustment":
		return parseAdjNext(args)
	case strings.HasPrefix(arg, "--adjustment="):
		return parseAdjValue(arg[len("--adjustment="):])
	case strings.HasPrefix(arg, "-n"):
		return parseAdjValue(arg[2:])
	default:
		return parseLegacyAdj(arg)
	}
	return 0, 0, nil // unreachable: --help/--version call os.Exit
}

// parseAdjNext parses -n N or --adjustment N where N is the next argument.
func parseAdjNext(args []string) (int, int, error) {
	if len(args) < 2 {
		return 0, 0, fmt.Errorf("option requires an argument -- 'n'")
	}
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid adjustment '%s'", args[1])
	}
	return 2, n, nil
}

// parseAdjValue parses an inline adjustment value string.
func parseAdjValue(val string) (int, int, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid adjustment '%s'", val)
	}
	return 1, n, nil
}

// parseLegacyAdj parses the legacy -N form (e.g., -5 means adjustment 5).
func parseLegacyAdj(arg string) (int, int, error) {
	val := arg[1:] // strip leading -
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid option '%s'", arg)
	}
	return 1, n, nil
}

// printNiceness prints the current scheduling priority to stdout.
// R1.3: when invoked with no COMMAND, print the current nice value.
func printNiceness() {
	prio, err := sys.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(exitInternal)
	}
	fmt.Println(prio)
}

// runCommand adjusts the scheduling priority and execs the command.
// R1.1: adjust priority by the given increment and exec COMMAND.
// R1.4: pass all remaining arguments to COMMAND.
func runCommand(adj int, cmd string, args []string) int {
	if err := adjustPriority(adj); err != nil {
		// R1.1: GNU nice warns on setpriority failure but continues to exec.
		fmt.Fprintf(os.Stderr, "%s: cannot set niceness: %s\n", progName, err)
	}
	path, err := exec.LookPath(cmd)
	if err != nil {
		// R2.2: exit 127 when COMMAND is not found.
		fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", progName, cmd, err)
		return exitNotFound
	}
	return execCommand(path, cmd, args)
}

// adjustPriority reads the current nice value and sets a new one.
func adjustPriority(adj int) error {
	prio, err := sys.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return err
	}
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, prio+adj)
}

// execCommand replaces the current process with the given command.
// R2.1: exit with COMMAND's exit status (handled by exec replacing the process).
// R2.2: exit 126 if COMMAND is found but cannot be invoked.
func execCommand(path, name string, args []string) int {
	argv := append([]string{name}, args...)
	err := syscall.Exec(path, argv, os.Environ())
	// syscall.Exec only returns on error.
	fmt.Fprintf(os.Stderr, "%s: '%s': %s\n", progName, name, err)
	return exitNoExec
}

// exitWithError prints an error message and exits 125.
// R2.2: exit 125 when nice itself fails.
func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	os.Exit(exitInternal)
}

// printHelp outputs usage information to stdout.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION] [COMMAND [ARG]...]\n", progName)
	fmt.Println("Run COMMAND with an adjusted niceness, which affects process scheduling.")
	fmt.Println("With no COMMAND, print the current niceness.")
	fmt.Println()
	fmt.Println("  -n, --adjustment=N   add integer N to the niceness (default 10)")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
}

// printVersion outputs version information to stdout.
func printVersion() {
	fmt.Printf("%s (go-unix-utils)\n", progName)
}
