// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/who shows who is logged on (prd097 R1.1–R1.4).
package main

/*
#include <utmpx.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const progName = "who"

func main() {
	installSIGPIPEHandler()

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
func run(args []string) error {
	amI, file, err := parseArgs(args)
	if err != nil {
		return err
	}

	// R1.2: When given a FILE argument, read that file as the utmpx database.
	if file != "" {
		if err := setUtmpxFile(file); err != nil {
			return err
		}
	}

	entries := readEntries()

	// R1.3: who am i — print only the entry for the current terminal.
	if amI {
		entries = filterCurrentTerminal(entries)
	}

	return printEntries(entries)
}

// parseArgs processes command-line arguments for R1.2 and R1.3.
func parseArgs(args []string) (amI bool, file string, err error) {
	// R1.3: "who am i" or "who am I" — two trailing words after optional file.
	if len(args) >= 2 {
		last2 := args[len(args)-2:]
		lower0 := strings.ToLower(last2[0])
		lower1 := strings.ToLower(last2[1])
		if lower0 == "am" && (lower1 == "i") {
			amI = true
			args = args[:len(args)-2]
		}
	}

	if len(args) > 1 {
		return false, "", fmt.Errorf("extra operand '%s'", args[1])
	}
	if len(args) == 1 {
		file = args[0]
	}
	return amI, file, nil
}

// setUtmpxFile sets the utmpx database path. R1.4: returns error if file
// cannot be accessed.
func setUtmpxFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("cannot open %s: %w", path, err)
	}
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	C.utmpxname(cs)
	return nil
}

// readEntries reads all USER_PROCESS utmpx entries.
func readEntries() []utmpxEntry {
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
		e := utmpxEntry{
			user: C.GoString(&entry.ut_user[0]),
			line: C.GoString(&entry.ut_line[0]),
			time: time.Unix(int64(entry.ut_tv.tv_sec), 0),
			host: C.GoString(&entry.ut_host[0]),
		}
		if e.user != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// filterCurrentTerminal returns only entries matching the current tty.
func filterCurrentTerminal(entries []utmpxEntry) []utmpxEntry {
	ttyPath, err := os.Readlink("/dev/fd/0")
	if err != nil {
		return nil
	}
	// Strip "/dev/" prefix to match ut_line.
	ttyLine := strings.TrimPrefix(ttyPath, "/dev/")

	var result []utmpxEntry
	for _, e := range entries {
		if e.line == ttyLine {
			result = append(result, e)
		}
	}
	return result
}

// printEntries formats and prints utmpx entries in GNU who format.
// R1.1: Each line: login_name terminal_line login_time (hostname)
func printEntries(entries []utmpxEntry) error {
	for _, e := range entries {
		line := formatEntry(e)
		if _, err := fmt.Println(line); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	return nil
}

// formatEntry formats a single utmpx entry in GNU who output format.
// Format: "user     line         YYYY-MM-DD HH:MM (host)"
func formatEntry(e utmpxEntry) string {
	timeStr := e.time.Format("2006-01-02 15:04")
	if e.host != "" {
		return fmt.Sprintf("%-8s %-12s %s (%s)", e.user, e.line, timeStr, e.host)
	}
	return fmt.Sprintf("%-8s %-12s %s", e.user, e.line, timeStr)
}

// installSIGPIPEHandler sets up SIGPIPE handling to exit 0, matching GNU coreutils.
// R3.3: Uses local implementation because pkg/sys may not be on disk during generation.
func installSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
