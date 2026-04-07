// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/uname: print system information.
// Implements srd044-uname R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "uname"

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -s, --kernel-name        print the kernel name
  -n, --nodename           print the network node hostname
  -r, --kernel-release     print the kernel release
      --help        display this help and exit
      --version     output version information and exit
`

// flags tracks which information fields the user requested.
type flags struct {
	sysName bool // -s: kernel name
	nodeName bool // -n: network node hostname
	release  bool // -r: kernel release
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	printFields(f)
}

// parseFlags processes command-line arguments and returns the selected flags.
// R1.1: when no flags are given, defaults to -s.
func parseFlags(args []string) (flags, error) {
	var f flags
	anySet := false

	for _, arg := range args {
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println(versionText)
			os.Exit(0)
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return f, fmt.Errorf("extra operand '%s'", arg)
		}
		if err := setShortFlags(&f, arg[1:]); err != nil {
			return f, err
		}
		anySet = true
	}

	if !anySet {
		f.sysName = true
	}
	return f, nil
}

// setShortFlags parses a group of short flag characters (e.g., "snr").
func setShortFlags(f *flags, chars string) error {
	for _, c := range chars {
		switch c {
		case 's':
			f.sysName = true
		case 'n':
			f.nodeName = true
		case 'r':
			f.release = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// printFields retrieves system info and prints the selected fields
// space-separated on a single line.
// R1.2: -s prints kernel name.
// R1.3: -n prints network node hostname.
// R1.4: -r prints kernel release string.
func printFields(f flags) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get system information: %v\n", progName, err)
		os.Exit(1)
	}

	var parts []string
	if f.sysName {
		parts = append(parts, bytesToString(utsname.Sysname[:]))
	}
	if f.nodeName {
		parts = append(parts, bytesToString(utsname.Nodename[:]))
	}
	if f.release {
		parts = append(parts, bytesToString(utsname.Release[:]))
	}

	fmt.Println(strings.Join(parts, " "))
}

// bytesToString converts a null-terminated byte array to a Go string.
func bytesToString(raw []byte) string {
	for i, v := range raw {
		if v == 0 {
			return string(raw[:i])
		}
	}
	return string(raw)
}
