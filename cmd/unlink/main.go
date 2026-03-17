// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd038-unlink R1.1-R1.3, R2.1-R2.4:
// cmd/unlink removes exactly one file via the unlink(2) system call.
// Rejects zero or multiple arguments and directories.
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU unlink format.
const progName = "unlink"

func main() {
	sys.InstallSIGPIPEHandler()

	args := parseArgs(os.Args[1:])

	if args.showVersion {
		fmt.Println("unlink (go-unix-utils) 0.1")
		os.Exit(0)
	}

	if args.showHelp {
		printUsage(os.Stdout)
		os.Exit(0)
	}

	// R1.1 / R2.1: exactly one operand required.
	if len(args.operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", os.Args[0])
		os.Exit(1)
	}

	// R2.2: more than one operand is an error.
	if len(args.operands) > 1 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, args.operands[1])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", os.Args[0])
		os.Exit(1)
	}

	// R1.1: call unlink(2) on the single operand.
	// Use syscall.Unlink instead of os.Remove because os.Remove falls back
	// to rmdir for directories, but unlink(2) must reject directories.
	name := args.operands[0]
	if err := syscall.Unlink(name); err != nil {
		// R2.3 / R2.4: print error matching GNU format with capitalized message.
		reason := capitalizeErrno(err)
		fmt.Fprintf(os.Stderr, "%s: cannot unlink '%s': %s\n", progName, name, reason)
		os.Exit(1)
	}

	// R1.2 / R1.3: no stdout output, exit 0.
}

// parsedArgs holds the result of argument parsing.
type parsedArgs struct {
	operands    []string
	showVersion bool
	showHelp    bool
}

// parseArgs separates flags from operand arguments.
func parseArgs(args []string) *parsedArgs {
	result := &parsedArgs{}
	flagsDone := false

	for i := range len(args) {
		arg := args[i]
		if flagsDone {
			result.operands = append(result.operands, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--version":
				result.showVersion = true
			case "--help":
				result.showHelp = true
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", os.Args[0])
				os.Exit(1)
			}
			continue
		}
		result.operands = append(result.operands, arg)
	}
	return result
}

// capitalizeErrno returns the errno error message with the first letter
// capitalized to match GNU coreutils error formatting (e.g., "No such file
// or directory" instead of Go's lowercase "no such file or directory").
func capitalizeErrno(err error) string {
	msg := err.Error()
	if len(msg) == 0 {
		return msg
	}
	// Capitalize the first rune.
	return strings.ToUpper(msg[:1]) + msg[1:]
}

// printUsage writes the help text to the given writer.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: unlink FILE")
	fmt.Fprintln(w, "  or:  unlink OPTION")
	fmt.Fprintln(w, "Call the unlink function to remove the specified FILE.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
}
