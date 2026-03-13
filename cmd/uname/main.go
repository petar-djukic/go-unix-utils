// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd044-uname R1.1, R1.2, R1.3, R1.4
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and help messages.
const programName = "uname"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// Parse flags manually to match GNU getopt behavior (combined flags like -snr).
	var flagS, flagN, flagR bool
	var positional []string

	for _, arg := range args {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags: parse each character.
			for _, ch := range arg[1:] {
				switch ch {
				case 's':
					flagS = true
				case 'n':
					flagN = true
				case 'r':
					flagR = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\nTry '%s --help' for more information.\n", programName, ch, programName)
					os.Exit(1)
				}
			}
		} else if arg == "--help" {
			fmt.Print(helpText)
			return
		} else if arg == "--version" {
			fmt.Println("uname (go-unix-utils) 0.1")
			return
		} else if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\nTry '%s --help' for more information.\n", programName, arg, programName)
			os.Exit(1)
		} else {
			positional = append(positional, arg)
		}
	}

	// R3.1: reject positional operands.
	if len(positional) > 0 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\nTry '%s --help' for more information.\n", programName, positional[0], programName)
		os.Exit(1)
	}

	// R1.1: no flags means print kernel name (equivalent to -s).
	if !flagS && !flagN && !flagR {
		flagS = true
	}

	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}

	// R2.2: print requested fields in canonical order, space-separated.
	var fields []string
	if flagS {
		// R1.2: kernel name.
		fields = append(fields, bytesToString(utsname.Sysname))
	}
	if flagN {
		// R1.3: network node hostname.
		fields = append(fields, bytesToString(utsname.Nodename))
	}
	if flagR {
		// R1.4: kernel release string.
		fields = append(fields, bytesToString(utsname.Release))
	}

	fmt.Println(strings.Join(fields, " "))
}

// bytesToString converts a null-terminated byte array to a Go string.
func bytesToString(field [256]byte) string {
	for i, b := range field {
		if b == 0 {
			return string(field[:i])
		}
	}
	return string(field[:])
}

// helpText is the usage message printed by --help.
const helpText = `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -s, --kernel-name        print the kernel name
  -n, --nodename           print the network node hostname
  -r, --kernel-release     print the kernel release
      --help     display this help and exit
      --version  output version information and exit
`
