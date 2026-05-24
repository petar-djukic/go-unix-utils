// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd097-who R1.1, R1.2, R1.3, R1.4.
package main

/*
#include <utmpx.h>
#include <unistd.h>
#include <stdlib.h>
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

const helpText = `Usage: who [OPTION]... [ FILE | ARG1 ARG2 ]
Print information about users who are currently logged in.

      --help     display this help and exit
      --version  output version information and exit

If FILE is not specified, use /var/run/utmpx.  /var/log/wtmp as FILE is common.
If ARG1 ARG2 given, -m presumed: 'am i' or 'mom likes' are usual.
`

const versionText = `who (go-unix-utils) dev
`

type whoEntry struct {
	user string
	line string
	time time.Time
	host string
}

func main() {
	sys.InstallSIGPIPEHandler()
	args, amI := parseArgs(os.Args[1:])

	if len(args) == 1 {
		if err := setUtmpxFile(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "who: %s: %v\n", args[0], err)
			os.Exit(1)
		}
	}

	entries := readEntries(amI)
	for _, e := range entries {
		if err := printEntry(e); err != nil {
			os.Exit(1)
		}
	}
}

func setUtmpxFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return pe.Err
		}
		return err
	}
	f.Close()
	cpath := C.CString(path)
	C.utmpxname(cpath)
	C.free(unsafe.Pointer(cpath))
	return nil
}

func readEntries(amI bool) []whoEntry {
	var myLine string
	if amI {
		myLine = currentLine()
		if myLine == "" {
			return nil
		}
	}

	C.setutxent()
	defer C.endutxent()

	var entries []whoEntry
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		if entry.ut_type != C.USER_PROCESS {
			continue
		}
		line := C.GoString(&entry.ut_line[0])
		if amI && line != myLine {
			continue
		}
		entries = append(entries, whoEntry{
			user: C.GoString(&entry.ut_user[0]),
			line: line,
			time: time.Unix(int64(entry.ut_tv.tv_sec), 0),
			host: C.GoString(&entry.ut_host[0]),
		})
	}
	return entries
}

func currentLine() string {
	tty := C.ttyname(C.int(os.Stdin.Fd()))
	if tty == nil {
		return ""
	}
	return strings.TrimPrefix(C.GoString(tty), "/dev/")
}

func printEntry(e whoEntry) error {
	timeStr := e.time.Format("2006-01-02 15:04")
	var line string
	if e.host != "" {
		line = fmt.Sprintf("%-8s %-12s %s (%s)", e.user, e.line, timeStr, e.host)
	} else {
		line = fmt.Sprintf("%-8s %-12s %s", e.user, e.line, timeStr)
	}
	_, err := fmt.Fprintln(os.Stdout, line)
	return err
}

func parseArgs(args []string) ([]string, bool) {
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
			fmt.Fprintf(os.Stderr, "who: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'who --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			fmt.Fprintf(os.Stderr, "who: invalid option -- '%c'\n", arg[1])
			fmt.Fprintln(os.Stderr, "Try 'who --help' for more information.")
			os.Exit(1)
		default:
			remaining = append(remaining, arg)
		}
	}
	if len(remaining) == 2 {
		return nil, true
	}
	if len(remaining) > 2 {
		fmt.Fprintf(os.Stderr, "who: extra operand '%s'\n", remaining[2])
		fmt.Fprintln(os.Stderr, "Try 'who --help' for more information.")
		os.Exit(1)
	}
	return remaining, false
}
