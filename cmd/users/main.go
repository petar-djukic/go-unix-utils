// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/users: print login names of logged-in users.
// Implements srd096-users R1.1-R1.3, R2.1-R2.3.
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

const progName = "users"

const versionText = progName + " (go-unix-utils)"

const helpText = `Usage: users [OPTION]... [FILE]
Output who is currently logged in according to FILE.
If FILE is not specified, use a default prescribed by the system.

      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	// R2.3: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}
}

// run processes arguments and prints sorted logged-in usernames.
// R1.1: prints space-separated sorted login names followed by a newline.
// R2.1: exits 0 on success.
func run(args []string) error {
	file, err := parseArgs(args)
	if err != nil {
		return err
	}

	names := readUsers(file)
	sort.Strings(names)

	if len(names) > 0 {
		fmt.Println(strings.Join(names, " "))
	}
	return nil
}

// parseArgs extracts an optional FILE argument and handles --help/--version.
// R1.2: supports optional FILE argument.
// R1.3: --help and --version flags.
func parseArgs(args []string) (string, error) {
	var file string
	for i, arg := range args {
		switch {
		case arg == "--help":
			fmt.Print(helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Println(versionText)
			os.Exit(0)
		case arg == "--":
			return parseFileAfterDash(args[i+1:], file)
		case strings.HasPrefix(arg, "-") && arg != "-":
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
			os.Exit(1)
		default:
			if file != "" {
				return "", fmt.Errorf("extra operand '%s'", arg)
			}
			file = arg
		}
	}
	return file, nil
}

// parseFileAfterDash handles remaining arguments after a -- separator.
func parseFileAfterDash(rest []string, existing string) (string, error) {
	file := existing
	for _, arg := range rest {
		if file != "" {
			return "", fmt.Errorf("extra operand '%s'", arg)
		}
		file = arg
	}
	return file, nil
}

// readUsers reads the utmpx database and returns login names.
// R1.1: reads system utmpx, filters USER_PROCESS entries.
// R1.2: reads FILE if non-empty.
// R1.3: duplicates are preserved (one entry per session).
func readUsers(file string) []string {
	if file != "" {
		cs := C.CString(file)
		C.utmpxname(cs)
		C.free(unsafe.Pointer(cs))
	}

	C.setutxent()
	defer C.endutxent()

	var names []string
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		// D3: filter to USER_PROCESS type only.
		if entry.ut_type != C.USER_PROCESS {
			continue
		}
		name := C.GoString(&entry.ut_user[0])
		names = append(names, name)
	}
	return names
}
