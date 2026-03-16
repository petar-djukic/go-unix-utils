// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd016-dirname R1.1-R1.5, R2.1-R2.2, R3.1-R3.3:
// cmd/dirname strips the last component from file paths, outputting the
// directory portion. Handles trailing slashes, root paths, no-slash paths,
// multiple operands, NUL-delimited output (--zero/-z), --help/--version
// flags, and error diagnostics. Installs SIGPIPE handler for clean exit
// on broken pipe.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in diagnostic output.
const progName = "dirname"

func main() {
	// R1.4 (task): install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var zeroTerminate bool

	// Parse flags manually to match GNU dirname behavior.
	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if arg == "--help" {
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s [OPTION] NAME...\nOutput each NAME with its last non-slash component and trailing slashes\nremoved; if NAME contains no /'s, output '.' (meaning the current directory).\n\n"+
					"  -z, --zero     end each output line with NUL, not newline\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName,
			)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}
		if arg == "--zero" {
			zeroTerminate = true
			continue
		}
		// Reject unrecognized long options.
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)     //nolint:errcheck // best-effort diagnostic
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
			os.Exit(1)
		}
		// Handle short flags, possibly combined (e.g., -z).
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'z':
					zeroTerminate = true
					j++
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, arg[j])    //nolint:errcheck // best-effort diagnostic
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
					os.Exit(1)
				}
			}
			continue
		}
		operands = append(operands, arg)
	}

	// R1.3 (task R3): exit 1 with diagnostic when no operands given.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)                   //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	terminator := "\n"
	if zeroTerminate {
		terminator = "\x00"
	}

	// R1.1: process each operand, printing the directory component.
	for _, name := range operands {
		result := dirname(name)
		fmt.Fprintf(os.Stdout, "%s%s", result, terminator) //nolint:errcheck // best-effort output
	}
}

// dirname strips the last component from name, matching GNU dirname behavior.
// R1.1: strip trailing slashes, then remove last component after final '/'.
// R1.2 (PRD): no '/' after trailing-slash removal returns ".".
// R1.3 (PRD): all-slash input returns "/".
// R1.4 (PRD): strip trailing slashes from result; empty result becomes "/".
func dirname(name string) string {
	// R1.3 (PRD): strip trailing slashes, but preserve at least one character.
	for len(name) > 1 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}

	// R1.2 (PRD): if no '/' remains, the directory is ".".
	lastSlash := strings.LastIndex(name, "/")
	if lastSlash < 0 {
		return "."
	}

	// Remove the last component (everything after the final '/').
	dir := name[:lastSlash]

	// R1.4 (PRD): strip trailing slashes from result.
	for len(dir) > 1 && dir[len(dir)-1] == '/' {
		dir = dir[:len(dir)-1]
	}

	// If stripping left nothing, this was a root-relative path like "/foo".
	if dir == "" {
		return "/"
	}

	return dir
}
