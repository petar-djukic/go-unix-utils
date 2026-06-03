// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd096-users R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

/*
#include <utmpx.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unsafe"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: users [OPTION]... [FILE]
Output who is currently logged in according to FILE.
If FILE is not specified, use /var/run/utmpx.  /var/log/wtmpx as FILE is common.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `users (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	args := parseArgs(os.Args[1:])

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "users: extra operand '%s'\n", args[1])
		fmt.Fprintln(os.Stderr, "Try 'users --help' for more information.")
		os.Exit(1)
	}

	if len(args) == 1 {
		cpath := C.CString(args[0])
		C.utmpxname(cpath)
		C.free(unsafe.Pointer(cpath))
	}

	names := readUsers()
	if len(names) > 0 {
		sort.Strings(names)
		if _, err := fmt.Fprintln(os.Stdout, strings.Join(names, " ")); err != nil {
			os.Exit(1)
		}
	}
}

func readUsers() []string {
	C.setutxent()
	defer C.endutxent()

	var names []string
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		if entry.ut_type != C.USER_PROCESS {
			continue
		}
		name := C.GoString(&entry.ut_user[0])
		names = append(names, name)
	}
	return names
}

func parseArgs(args []string) []string {
	var remaining []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			remaining = append(remaining, arg)
			continue
		}
		switch {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--":
			endOfFlags = true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "users: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'users --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			fmt.Fprintf(os.Stderr, "users: invalid option -- '%c'\n", arg[1])
			fmt.Fprintln(os.Stderr, "Try 'users --help' for more information.")
			os.Exit(1)
		default:
			remaining = append(remaining, arg)
		}
	}
	return remaining
}
