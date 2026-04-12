// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/who: show who is logged on.
// Implements srd097-who R1.1-R1.4: core utmpx reading and default output.
// Implements srd097-who R2.1-R2.4: flags and display options.
// Implements srd097-who R3.1-R3.3: error handling, version, and help.
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

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "who"

// timeFormat matches GNU who output: "Jan _2 HH:MM".
const timeFormat = "Jan _2 15:04"

// utmpEntry holds one session from the utmpx database.
type utmpEntry struct {
	user    string
	line    string
	time    time.Time
	host    string
	pid     int
	entType int
}

// options holds parsed command-line flags.
type options struct {
	heading bool // R2.1: -H/--heading
	users   bool // R2.2: -u/--users
	boot    bool // R2.3: -b/--boot
	count   bool // R2.4: -q/--count
	version bool // R3.1: --version
	help    bool // R3.2: --help
	amI     bool // R1.3: "am i" two-argument form
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	if opts.version {
		printVersion()
		return
	}
	if opts.help {
		printUsage()
		return
	}

	entries := readAllEntries()
	if opts.amI {
		entries = filterCurrentUser(entries)
	}
	displayEntries(entries, opts)
}

// showUsers returns true when user-process entries should be printed.
// When no entry-type flag is set, users are shown by default.
// When -b is set alone, only boot entries are shown.
func showUsers(opts options) bool {
	if opts.users {
		return true
	}
	return !opts.boot
}

// displayEntries dispatches to the appropriate output mode.
func displayEntries(entries []utmpEntry, opts options) {
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
	if showUsers(opts) {
		printUserEntries(entries, opts)
	}
}

// parseArgs parses command-line arguments into options.
// Returns an error for unrecognized flags (R3.2).
func parseArgs(args []string) (options, error) {
	var opts options
	var nonFlags []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			nonFlags = append(nonFlags, arg)
			continue
		}
		if err := applyFlag(&opts, arg); err != nil {
			return options{}, err
		}
	}
	// R1.3: any two non-flag arguments trigger "am i" mode.
	if len(nonFlags) >= 2 {
		opts.amI = true
	}
	return opts, nil
}

// applyFlag sets the option corresponding to arg, or returns an error.
func applyFlag(opts *options, arg string) error {
	switch arg {
	case "-H", "--heading":
		opts.heading = true
	case "-u", "--users":
		opts.users = true
	case "-b", "--boot":
		opts.boot = true
	case "-q", "--count":
		opts.count = true
	case "--version":
		opts.version = true
	case "--help":
		opts.help = true
	default:
		return fmt.Errorf("%s: unrecognized option '%s'\nTry '%s --help' for more information.",
			progName, arg, progName)
	}
	return nil
}

// printVersion prints version information to stdout and exits 0.
// R3.1: matches GNU who --version output structure.
func printVersion() {
	fmt.Printf("%s (go-unix-utils) 0.1\n", progName)
}

// printUsage prints usage information to stdout.
// R3.2: matches GNU who --help output structure.
func printUsage() {
	fmt.Printf("Usage: %s [OPTION]... [ FILE | ARG1 ARG2 ]\n", progName)
	fmt.Println()
	fmt.Println("Print information about users who are currently logged in.")
	fmt.Println()
	fmt.Println("  -b, --boot     time of last system boot")
	fmt.Println("  -H, --heading  print line of column headings")
	fmt.Println("  -q, --count    all login names and number of users logged on")
	fmt.Println("  -u, --users    add user idle time")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
}

// currentTTY returns the terminal name for stdin, stripped of "/dev/" prefix.
// D1: used by "am i" form to identify the current session.
func currentTTY() string {
	name := C.ttyname(0)
	if name == nil {
		return ""
	}
	return strings.TrimPrefix(C.GoString(name), "/dev/")
}

// filterCurrentUser returns only the current user's session and boot entries.
// D1: if stdin is not a terminal, returns nil (print nothing, exit 0).
func filterCurrentUser(entries []utmpEntry) []utmpEntry {
	tty := currentTTY()
	if tty == "" {
		return nil
	}
	var filtered []utmpEntry
	for _, e := range entries {
		if e.entType == C.BOOT_TIME {
			filtered = append(filtered, e)
			continue
		}
		if e.entType == C.USER_PROCESS && e.line == tty {
			filtered = append(filtered, e)
		}
	}
	return filtered
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
		pid:     int(entry.ut_pid),
		entType: eType,
	}
}

// printHeading prints a header line above the output.
// R2.1: header shows NAME, LINE, TIME, and optionally IDLE, COMMENT.
func printHeading(opts options) {
	if opts.users {
		fmt.Printf("%-8s %-12s %-12s %-5s %12s %s\n",
			"NAME", "LINE", "TIME", "IDLE", "PID", "COMMENT")
	} else {
		fmt.Printf("%-8s %-12s %-12s %s\n",
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
			timeStr := e.time.Format(timeFormat)
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
// R1.1: format is NAME LINE TIME (HOST).
func printEntry(e utmpEntry) {
	timeStr := e.time.Format(timeFormat)
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s (%s)\n", e.user, e.line, timeStr, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s\n", e.user, e.line, timeStr)
	}
}

// printEntryWithIdle prints a user session line including idle time and PID.
// R2.2: idle time as HH:MM, '.' for active, 'old' for > 24 hours.
// Format matches GNU who -u: NAME LINE TIME IDLE PID (HOST).
func printEntryWithIdle(e utmpEntry) {
	timeStr := e.time.Format(timeFormat)
	idle := idleString(e.line)
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s %-5s%12d (%s)\n",
			e.user, e.line, timeStr, idle, e.pid, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s %-5s%12d\n",
			e.user, e.line, timeStr, idle, e.pid)
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
