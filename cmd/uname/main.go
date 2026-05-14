// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd044-uname R1.1-R1.9, R2.1-R2.2.
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

  -a                print all information
  -s                print the kernel name
  -n                print the network node hostname
  -r                print the kernel release
  -v                print the kernel version
  -m                print the machine hardware name
  -p                print the processor type or "unknown"
  -i                print the hardware platform or "unknown"
  -o                print the operating system
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `uname (go-unix-utils) dev
`

type fieldEntry struct {
	flag byte
	get  func(*unix.Utsname) string
}

func processorType(u *unix.Utsname) string {
	if utsToString(u.Sysname[:]) == "Darwin" {
		switch utsToString(u.Machine[:]) {
		case "arm64":
			return "arm"
		case "x86_64":
			return "i386"
		}
	}
	return "unknown"
}

var fields = []fieldEntry{
	{'s', func(u *unix.Utsname) string { return utsToString(u.Sysname[:]) }},
	{'n', func(u *unix.Utsname) string { return utsToString(u.Nodename[:]) }},
	{'r', func(u *unix.Utsname) string { return utsToString(u.Release[:]) }},
	{'v', func(u *unix.Utsname) string { return utsToString(u.Version[:]) }},
	{'m', func(u *unix.Utsname) string { return utsToString(u.Machine[:]) }},
	{'p', processorType},
	{'i', func(_ *unix.Utsname) string { return "unknown" }},
	{'o', func(u *unix.Utsname) string {
		sysname := utsToString(u.Sysname[:])
		if sysname == "Linux" {
			return "GNU/Linux"
		}
		return sysname
	}},
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

func parseFlags(args []string) ([]bool, bool, []string) {
	selected := make([]bool, len(fields))
	var allFlag bool
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
			return selected, allFlag, args[1:]
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(os.Stderr, "uname: unrecognized option '%s'\n", a)
			fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for i := 1; i < len(a); i++ {
				if a[i] == 'a' {
					for j := range selected {
						selected[j] = true
					}
					allFlag = true
					continue
				}
				idx, ok := flagIndex[a[i]]
				if !ok {
					fmt.Fprintf(os.Stderr, "uname: invalid option -- '%c'\n", a[i])
					fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
					os.Exit(1)
				}
				selected[idx] = true
			}
		default:
			return selected, allFlag, args
		}
		args = args[1:]
	}
	return selected, allFlag, args
}

func formatOutput(u *unix.Utsname, selected []bool, allFlag bool) string {
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
			val := f.get(u)
			if allFlag && val == "unknown" {
				continue
			}
			parts = append(parts, val)
		}
	}
	return strings.Join(parts, " ")
}

func main() {
	sys.InstallSIGPIPEHandler()

	selected, allFlag, args := parseFlags(os.Args[1:])

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

	if _, err := fmt.Fprintln(os.Stdout, formatOutput(&u, selected, allFlag)); err != nil {
		os.Exit(1)
	}
}
