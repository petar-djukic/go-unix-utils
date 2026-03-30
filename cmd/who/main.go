// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/who implements prd097-who R1.1, R1.2, R1.3, R1.4:
// show who is logged on by reading the utmpx database.

package main

/*
#include <utmpx.h>
#include <stdlib.h>
#include <unistd.h>

// utmpxname is available on macOS but not declared in all header versions.
extern int utmpxname(const char *);
*/
import "C"

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the binary name used in error messages.
const progName = "who"

func main() {
	// R3.3: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}
}

// utmpxEntry holds the fields extracted from a utmpx record.
type utmpxEntry struct {
	user string
	line string
	time time.Time
	host string
}

// run parses arguments and prints who output.
// R1.1: default user listing.
// R1.2: optional FILE argument.
// R1.3: "who am i" mode.
// R1.4: exit 1 on error.
func run(args []string) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.utmpxFile != "" {
		setUtmpxFile(opts.utmpxFile)
	}
	entries := readUtmpxEntries()
	if opts.amI {
		printAmI(entries)
	} else {
		printEntries(entries)
	}
	return nil
}

// options holds parsed command-line options for the who command.
type options struct {
	utmpxFile string
	amI       bool
}

// parseArgs extracts options from command-line arguments.
// R1.2: FILE argument detection.
// R1.3: "am i" detection (any two non-option operands).
func parseArgs(args []string) (options, error) {
	var opts options
	var operands []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return opts, fmt.Errorf("unrecognized option '%s'", arg)
		}
		operands = append(operands, arg)
	}
	return classifyOperands(opts, operands)
}

// classifyOperands determines mode from positional arguments.
func classifyOperands(opts options, operands []string) (options, error) {
	switch len(operands) {
	case 0:
		// R1.1: default listing.
	case 1:
		// R1.2: single argument is a FILE.
		opts.utmpxFile = operands[0]
	case 2:
		// R1.3: two non-option arguments triggers "am i" mode.
		opts.amI = true
	default:
		return opts, fmt.Errorf("extra operand '%s'", operands[2])
	}
	return opts, nil
}

// setUtmpxFile sets the utmpx database file to read from.
// R1.2: uses utmpxname(3) to override the default database path.
func setUtmpxFile(path string) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	C.utmpxname(cpath)
}

// readUtmpxEntries reads all USER_PROCESS entries from the utmpx database.
// R1.1: collects login name, terminal, time, and hostname.
func readUtmpxEntries() []utmpxEntry {
	C.setutxent()
	defer C.endutxent()
	var entries []utmpxEntry
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		if entry.ut_type != C.USER_PROCESS {
			continue
		}
		entries = append(entries, extractEntry(entry))
	}
	return entries
}

// extractEntry converts a C utmpx entry to a Go utmpxEntry.
func extractEntry(entry *C.struct_utmpx) utmpxEntry {
	return utmpxEntry{
		user: C.GoString(&entry.ut_user[0]),
		line: C.GoString(&entry.ut_line[0]),
		time: time.Unix(int64(entry.ut_tv.tv_sec), 0),
		host: C.GoString(&entry.ut_host[0]),
	}
}

// printEntries prints all utmpx entries in GNU who format.
// R1.1: one line per logged-in user.
func printEntries(entries []utmpxEntry) {
	for _, e := range entries {
		printOneLine(e)
	}
}

// printAmI prints only the entry matching the current terminal.
// R1.3: "who am i" prints the entry for the caller's terminal.
func printAmI(entries []utmpxEntry) {
	ttyName := currentTTY()
	for _, e := range entries {
		if e.line == ttyName {
			printOneLine(e)
			return
		}
	}
}

// currentTTY returns the terminal name for the current process,
// stripped of the "/dev/" prefix to match utmpx ut_line values.
func currentTTY() string {
	name := C.ttyname(C.STDIN_FILENO)
	if name == nil {
		return ""
	}
	full := C.GoString(name)
	return strings.TrimPrefix(full, "/dev/")
}

// printOneLine formats and prints a single utmpx entry in GNU who format.
// R1.1: format matches GNU who: "%-8s %-12s %s".
func printOneLine(e utmpxEntry) {
	timeStr := e.time.Format("Jan _2 15:04")
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s (%s)\n", e.user, e.line, timeStr, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s\n", e.user, e.line, timeStr)
	}
}
