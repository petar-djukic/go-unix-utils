// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/users implements prd096-users: print login names of currently logged-in
// users, sorted alphabetically and space-separated on a single line.

package main

/*
#include <utmpx.h>
#include <stdlib.h>

// utmpxname is available on macOS but not declared in all header versions.
// We declare it explicitly to avoid implicit-function-declaration warnings.
extern int utmpxname(const char *);
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

// progName is the binary name used in error messages.
const progName = "users"

func main() {
	// R2.3: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: exit 0 on success, 1 on error.
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
}

// run implements the core users logic.
// R1.1: reads utmpx, prints sorted space-separated login names.
// R1.2: accepts optional FILE argument for alternate utmpx database.
func run(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("extra operand '%s'", args[1])
	}
	if len(args) == 1 {
		if err := setUtmpxFile(args[0]); err != nil {
			return err
		}
	}
	names := readLoginNames()
	if len(names) > 0 {
		sort.Strings(names)
		fmt.Println(strings.Join(names, " "))
	}
	return nil
}

// setUtmpxFile sets the utmpx database file to read from.
// R1.2: uses utmpxname(3) to override the default database path.
func setUtmpxFile(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	rc := C.utmpxname(cpath)
	if rc != 0 {
		return fmt.Errorf("cannot open '%s'", path)
	}
	return nil
}

// readLoginNames reads the utmpx database and returns login names for all
// USER_PROCESS entries.
// R1.1: collects all logged-in user names.
// R1.3: duplicates are preserved (one entry per session).
func readLoginNames() []string {
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
