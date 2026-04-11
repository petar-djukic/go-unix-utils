// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/who: show who is logged on.
// Implements srd097-who R1.1-R1.4: core utmpx reading and default output.
// Implements srd097-who R2.1-R2.4: flags and display options.
package main

/*
#include <utmpx.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "who"

// utmpEntry holds one session from the utmpx database.
type utmpEntry struct {
	user    string
	line    string
	time    time.Time
	host    string
	entType int
}

// options holds parsed command-line flags.
type options struct {
	heading bool // R2.1: -H/--heading
	users   bool // R2.2: -u/--users
	boot    bool // R2.3: -b/--boot
	count   bool // R2.4: -q/--count
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])
	entries := readAllEntries()

	if opts.count {
		printCount(entries)
		return
	}
	if opts.heading {
		printHeading(opts)
	}
	if opts.boot {
		printBootEntry(entries)
	}
	printUserEntries(entries, opts)
}

// parseArgs parses command-line arguments into options.
// D1: supports both short and long flag forms.
func parseArgs(args []string) options {
	var opts options
	for _, arg := range args {
		switch arg {
		case "-H", "--heading":
			opts.heading = true
		case "-u", "--users":
			opts.users = true
		case "-b", "--boot":
			opts.boot = true
		case "-q", "--count":
			opts.count = true
		}
	}
	return opts
}

// readAllEntries reads the utmpx database and returns all relevant entries.
func readAllEntries() []utmpEntry {
	C.setutxent()
	defer C.endutxent()

	var entries []utmpEntry
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		eType := int(entry.ut_type)
		if eType != C.USER_PROCESS && eType != C.BOOT_TIME {
			continue
		}
		entries = append(entries, extractEntry(entry, eType))
	}
	return entries
}

// extractEntry converts a C utmpx entry to a Go utmpEntry.
func extractEntry(entry *C.struct_utmpx, eType int) utmpEntry {
	return utmpEntry{
		user:    C.GoString(&entry.ut_user[0]),
		line:    C.GoString(&entry.ut_line[0]),
		time:    time.Unix(int64(entry.ut_tv.tv_sec), 0),
		host:    C.GoString(&entry.ut_host[0]),
		entType: eType,
	}
}

// printHeading prints a header line above the output.
// R2.1: header shows NAME, LINE, TIME, and optionally IDLE, COMMENT.
func printHeading(opts options) {
	if opts.users {
		fmt.Printf("%-8s %-12s %-5s %-16s %s\n",
			"NAME", "LINE", "IDLE", "TIME", "COMMENT")
	} else {
		fmt.Printf("%-8s %-12s %-16s %s\n",
			"NAME", "LINE", "TIME", "COMMENT")
	}
}

// printCount prints login names and a count of logged-in users.
// R2.4: -q overrides other display flags (D2).
func printCount(entries []utmpEntry) {
	var names []string
	for _, e := range entries {
		if e.entType == C.USER_PROCESS {
			names = append(names, e.user)
		}
	}
	if len(names) > 0 {
		fmt.Println(strings.Join(names, " "))
	}
	fmt.Printf("# users=%d\n", len(names))
}

// printBootEntry prints the last system boot time.
// R2.3: reads the BOOT_TIME entry from the utmpx database.
func printBootEntry(entries []utmpEntry) {
	for _, e := range entries {
		if e.entType == C.BOOT_TIME {
			timeStr := e.time.Format("2006-01-02 15:04")
			fmt.Printf("%-8s %-12s %s\n", "", "system boot", timeStr)
			return
		}
	}
}

// printUserEntries prints user session lines.
func printUserEntries(entries []utmpEntry, opts options) {
	for _, e := range entries {
		if e.entType != C.USER_PROCESS {
			continue
		}
		if opts.users {
			printEntryWithIdle(e)
		} else {
			printEntry(e)
		}
	}
}

// printEntry prints one user session line in GNU who default format.
// R1.2: format is NAME LINE TIME (HOST).
func printEntry(e utmpEntry) {
	timeStr := e.time.Format("2006-01-02 15:04")
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s (%s)\n", e.user, e.line, timeStr, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s\n", e.user, e.line, timeStr)
	}
}

// printEntryWithIdle prints a user session line including idle time.
// R2.2: idle time as HH:MM, '.' for active, 'old' for > 24 hours.
func printEntryWithIdle(e utmpEntry) {
	timeStr := e.time.Format("2006-01-02 15:04")
	idle := idleString(e.line)
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s %s (%s)\n",
			e.user, e.line, idle, timeStr, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s %s\n",
			e.user, e.line, idle, timeStr)
	}
}

// idleString computes the idle time string for a terminal device.
// R2.2: '.' for < 1 minute, HH:MM for 1 min to 24 hours, 'old' for > 24 hours.
func idleString(line string) string {
	devPath := "/dev/" + line
	info, err := os.Stat(devPath)
	if err != nil {
		return "  ?  "
	}
	idle := time.Since(info.ModTime())
	if idle < time.Minute {
		return "  .  "
	}
	const day = 24 * time.Hour
	if idle >= day {
		return " old "
	}
	hours := int(idle.Hours())
	minutes := int(idle.Minutes()) % 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
