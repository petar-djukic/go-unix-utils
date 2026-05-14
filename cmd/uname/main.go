// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd044-uname R1.1-R1.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"golang.org/x/sys/unix"
)

const helpText = `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -s                print the kernel name
  -n                print the network node hostname
  -r                print the kernel release
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `uname (go-unix-utils) dev
`

type fieldEntry struct {
	flag byte
	get  func(*unix.Utsname) string
}

var fields = []fieldEntry{
	{'s', func(u *unix.Utsname) string { return utsToString(u.Sysname[:]) }},
	{'n', func(u *unix.Utsname) string { return utsToString(u.Nodename[:]) }},
	{'r', func(u *unix.Utsname) string { return utsToString(u.Release[:]) }},
}

var flagIndex map[byte]int

func init() {
	flagIndex = make(map[byte]int, len(fields))
	for i, f := range fields {
		flagIndex[f.flag] = i
	}
}

func utsToString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func parseFlags(args []string) ([]bool, []string) {
	selected := make([]bool, len(fields))
	for len(args) > 0 {
		a := args[0]
		switch {
		case a == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case a == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case a == "--":
			return selected, args[1:]
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(os.Stderr, "uname: unrecognized option '%s'\n", a)
			fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for i := 1; i < len(a); i++ {
				idx, ok := flagIndex[a[i]]
				if !ok {
					fmt.Fprintf(os.Stderr, "uname: invalid option -- '%c'\n", a[i])
					fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
					os.Exit(1)
				}
				selected[idx] = true
			}
		default:
			return selected, args
		}
		args = args[1:]
	}
	return selected, args
}

func formatOutput(u *unix.Utsname, selected []bool) string {
	hasSelection := false
	for _, s := range selected {
		if s {
			hasSelection = true
			break
		}
	}
	if !hasSelection {
		selected[0] = true
	}
	var parts []string
	for i, f := range fields {
		if selected[i] {
			parts = append(parts, f.get(u))
		}
	}
	return strings.Join(parts, " ")
}

func main() {
	sys.InstallSIGPIPEHandler()

	selected, args := parseFlags(os.Args[1:])

	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "uname: extra operand '%s'\n", args[0])
		fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
		os.Exit(1)
	}

	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		fmt.Fprintf(os.Stderr, "uname: cannot get system information: %v\n", err)
		os.Exit(1)
	}

	if _, err := fmt.Fprintln(os.Stdout, formatOutput(&u, selected)); err != nil {
		os.Exit(1)
	}
}
