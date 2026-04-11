// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/pinky: lightweight finger information lookup.
// Implements srd098-pinky R1.1-R1.3: core utmpx reading and default output.
package main

/*
#include <utmpx.h>
*/
import "C"

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "pinky"

// timeFormat matches GNU pinky short-format login time.
const timeFormat = "2006-01-02 15:04"

// utmpEntry holds one user session from the utmpx database.
type utmpEntry struct {
	user string
	line string
	time time.Time
	host string
}

// main is the entry point for cmd/pinky.
// R1.1: reads utmpx and displays logged-in users in short format.
// R1.2: filters by user operands when given.
func main() {
	// D1/R3.3: install SIGPIPE handler per project convention.
	sys.InstallSIGPIPEHandler()

	operands := parseArgs(os.Args[1:])
	entries := readUserEntries()
	if len(operands) > 0 {
		entries = filterByUsers(entries, operands)
	}
	printHeader()
	printEntries(entries)
}

// parseArgs extracts non-flag user operands from command-line arguments.
// R1.3: -s forces short format (already default), accepted silently.
func parseArgs(args []string) []string {
	var operands []string
	for _, arg := range args {
		if arg == "-s" {
			// R1.3: short format is already default.
			continue
		}
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			reportUnrecognizedOption(arg)
		}
		operands = append(operands, arg)
	}
	return operands
}

// reportUnrecognizedOption prints an error for an unknown flag and exits.
func reportUnrecognizedOption(arg string) {
	fmt.Fprintf(os.Stderr, "%s: invalid option -- '%s'\n",
		progName, strings.TrimLeft(arg, "-"))
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	os.Exit(1)
}

// readUserEntries reads the utmpx database and returns USER_PROCESS entries.
// R1.1: filters to USER_PROCESS type only.
func readUserEntries() []utmpEntry {
	C.setutxent()
	defer C.endutxent()

	var entries []utmpEntry
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

// extractEntry converts a C utmpx entry to a Go utmpEntry.
func extractEntry(entry *C.struct_utmpx) utmpEntry {
	return utmpEntry{
		user: C.GoString(&entry.ut_user[0]),
		line: C.GoString(&entry.ut_line[0]),
		time: time.Unix(int64(entry.ut_tv.tv_sec), 0),
		host: C.GoString(&entry.ut_host[0]),
	}
}

// filterByUsers returns only entries whose login name matches one of the operands.
// R1.2: restrict output to users matching given operands.
func filterByUsers(entries []utmpEntry, users []string) []utmpEntry {
	set := make(map[string]bool, len(users))
	for _, u := range users {
		set[u] = true
	}
	var filtered []utmpEntry
	for _, e := range entries {
		if set[e.user] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// printHeader prints the short-format column header line.
func printHeader() {
	fmt.Printf("%-8s %-19s %-9s %-5s %-16s %s\n",
		"Login", "Name", "TTY", "Idle", "When", "Where")
}

// printEntries prints all user entries in short format.
func printEntries(entries []utmpEntry) {
	for _, e := range entries {
		printShortEntry(e)
	}
}

// printShortEntry prints one user entry in short format.
// R1.1: shows login name, full name, tty, idle, login time, remote host.
func printShortEntry(e utmpEntry) {
	fullName := lookupFullName(e.user)
	idle := idleString(e.line)
	timeStr := e.time.Format(timeFormat)
	fmt.Printf("%-8s %-19s %-9s %-5s %s %s\n",
		e.user, fullName, e.line, idle, timeStr, e.host)
}

// lookupFullName returns the GECOS full name for the given username.
// Returns empty string if the user is not found.
func lookupFullName(username string) string {
	u, err := user.Lookup(username)
	if err != nil {
		return ""
	}
	// GECOS may contain comma-separated fields; the first is the full name.
	name := u.Name
	if idx := strings.Index(name, ","); idx >= 0 {
		name = name[:idx]
	}
	return name
}

// idleString computes the idle time string for a terminal device.
// Returns " " for active (< 1 min), "HH:MM" otherwise, "old" for > 24h.
func idleString(line string) string {
	devPath := "/dev/" + line
	info, err := os.Stat(devPath)
	if err != nil {
		return "  ?  "
	}
	idle := time.Since(info.ModTime())
	if idle < time.Minute {
		return "     "
	}
	const day = 24 * time.Hour
	if idle >= day {
		return " old "
	}
	hours := int(idle.Hours())
	minutes := int(idle.Minutes()) % 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
