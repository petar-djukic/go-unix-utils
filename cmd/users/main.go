// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd096-users: Print Login Names of Currently Logged-In Users.
// Covers R1.1-R1.3 (utmpx reading, user filtering, sorted output),
// R2.1-R2.3 (exit codes, SIGPIPE handling).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints logged-in usernames. Returns exit code.
func run(args []string) int {
	file, exitCode := parseArgs(args)
	if exitCode >= 0 {
		return exitCode
	}

	users, err := readUsers(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "users: %v\n", err)
		return 1
	}

	return printUsers(users)
}

// parseArgs extracts the optional FILE argument from the command line.
// Returns (file, -1) on success, or ("", exitCode) to exit immediately.
func parseArgs(args []string) (string, int) {
	var file string
	pastFlags := false
	for _, arg := range args {
		if !pastFlags && arg == "--" {
			pastFlags = true
			continue
		}
		if !pastFlags {
			if arg == "--help" {
				return "", printHelp()
			}
			if arg == "--version" {
				return "", printVersion()
			}
			if len(arg) > 1 && arg[0] == '-' {
				fmt.Fprintf(os.Stderr,
					"users: unrecognized option '%s'\n", arg)
				return "", 1
			}
		}
		if file != "" {
			fmt.Fprintf(os.Stderr,
				"users: extra operand '%s'\n", arg)
			return "", 1
		}
		file = arg
	}
	return file, -1
}

// readUsers reads the utmpx database and returns usernames from
// USER_PROCESS entries. R1.1, R1.2.
func readUsers(file string) ([]string, error) {
	if file != "" {
		if err := setUtmpxFile(file); err != nil {
			return nil, err
		}
	}

	C.setutxent()
	defer C.endutxent()

	var users []string
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		// R1.2: filter for USER_PROCESS entries only.
		if entry.ut_type != C.USER_PROCESS {
			continue
		}
		// R1.3: include duplicates (same user, multiple sessions).
		name := C.GoString(&entry.ut_user[0])
		if name != "" {
			users = append(users, name)
		}
	}
	return users, nil
}

// setUtmpxFile configures the utmpx library to read from a specific file.
// R1.2: accept an optional FILE argument.
func setUtmpxFile(file string) error {
	if _, err := os.Stat(file); err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return fmt.Errorf("%s: %v", pe.Path, pe.Err)
		}
		return err
	}
	cfile := C.CString(file)
	defer C.free(unsafe.Pointer(cfile))
	C.utmpxname(cfile)
	return nil
}

// printUsers sorts and prints space-separated usernames. R1.1, R1.3.
func printUsers(users []string) int {
	if len(users) == 0 {
		return 0
	}
	sort.Strings(users)
	if _, err := fmt.Println(strings.Join(users, " ")); err != nil {
		return 1
	}
	return 0
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: users [OPTION]... [FILE]
Output who is currently logged in according to FILE.
If FILE is not specified, use a default utmpx file.

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "users (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
