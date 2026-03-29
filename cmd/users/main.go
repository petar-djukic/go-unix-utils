// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/users prints the login names of currently logged-in users (prd096 R1.1–R2.1).
package main

/*
#include <utmpx.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

func main() {
	installSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "users: %v\n", err)
		os.Exit(1)
	}
}

// run parses arguments and prints logged-in usernames.
func run(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("extra operand '%s'", args[1])
	}

	// R1.2: When given a FILE argument, read that file as the utmpx database.
	if len(args) == 1 {
		cs := C.CString(args[0])
		defer C.free(unsafe.Pointer(cs))
		C.utmpxname(cs)
	}

	names := readUsers()
	// R1.1: Sort alphabetically.
	sort.Strings(names)

	// R1.1: Print space-separated on a single line.
	if len(names) > 0 {
		fmt.Println(strings.Join(names, " "))
	}

	// R2.1: Exit 0 on success.
	return nil
}

// readUsers reads utmpx entries and returns login names for user processes.
// R1.3: Duplicates are preserved — one entry per session.
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
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// installSIGPIPEHandler sets up SIGPIPE handling to exit 0, matching GNU coreutils.
// R2.3: Equivalent to sys.InstallSIGPIPEHandler() when pkg/sys is available.
func installSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
