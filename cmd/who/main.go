// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/who implements prd097-who R1.1-R1.4, R2.1-R2.4, R3.1-R3.3:
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
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
}

// utmpxEntry holds the fields extracted from a utmpx record.
type utmpxEntry struct {
	user    string
	line    string
	time    time.Time
	host    string
	pid     int
	entType int
}

// options holds parsed command-line options for the who command.
type options struct {
	utmpxFile string
	amI       bool
	heading   bool // R2.1: -H/--heading
	showIdle  bool // R2.2: -u/--users
	boot      bool // R2.3: -b/--boot
	count     bool // R2.4: -q/--count
}

// run parses arguments and prints who output.
func run(args []string) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.utmpxFile != "" {
		setUtmpxFile(opts.utmpxFile)
	}
	entries := readUtmpxEntries(opts)
	printOutput(entries, opts)
	return nil
}

// parseArgs extracts options from command-line arguments.
// R1.2: FILE argument detection.
// R1.3: "am i" detection (any two non-option operands).
// R2.1-R2.4: flag parsing for display options.
func parseArgs(args []string) (options, error) {
	var opts options
	var operands []string
	for i, arg := range args {
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			if err := applyLongFlag(&opts, arg); err != nil {
				return opts, err
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if err := applyShortFlags(&opts, arg); err != nil {
				return opts, err
			}
			continue
		}
		operands = append(operands, arg)
	}
	return classifyOperands(opts, operands)
}

// applyLongFlag handles a single --flag argument.
func applyLongFlag(opts *options, arg string) error {
	switch arg {
	case "--heading":
		opts.heading = true
	case "--users":
		opts.showIdle = true
	case "--boot":
		opts.boot = true
	case "--count":
		opts.count = true
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}

// applyShortFlags handles combined short flags like -Hbu.
func applyShortFlags(opts *options, arg string) error {
	for _, ch := range arg[1:] {
		switch ch {
		case 'H':
			opts.heading = true
		case 'u':
			opts.showIdle = true
		case 'b':
			opts.boot = true
		case 'q':
			opts.count = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
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

// readUtmpxEntries reads entries from the utmpx database.
// Collects USER_PROCESS entries, and BOOT_TIME entries when -b is set.
func readUtmpxEntries(opts options) []utmpxEntry {
	C.setutxent()
	defer C.endutxent()
	var entries []utmpxEntry
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		if shouldCollect(entry, opts) {
			entries = append(entries, extractEntry(entry))
		}
	}
	return entries
}

// shouldCollect returns true if the utmpx entry should be included.
// When type flags (-b, -u) are given, only requested types are shown.
// Default (no type flags) shows USER_PROCESS. Count mode always needs users.
func shouldCollect(entry *C.struct_utmpx, opts options) bool {
	if entry.ut_type == C.USER_PROCESS {
		return opts.count || !opts.boot || opts.showIdle
	}
	return opts.boot && entry.ut_type == C.BOOT_TIME
}

// extractEntry converts a C utmpx entry to a Go utmpxEntry.
func extractEntry(entry *C.struct_utmpx) utmpxEntry {
	return utmpxEntry{
		user:    C.GoString(&entry.ut_user[0]),
		line:    C.GoString(&entry.ut_line[0]),
		time:    time.Unix(int64(entry.ut_tv.tv_sec), 0),
		host:    C.GoString(&entry.ut_host[0]),
		pid:     int(entry.ut_pid),
		entType: int(entry.ut_type),
	}
}

// printOutput dispatches to the appropriate output mode.
func printOutput(entries []utmpxEntry, opts options) {
	if opts.count {
		printCount(entries)
		return
	}
	if opts.heading {
		printHeading(opts)
	}
	if opts.amI {
		printAmI(entries, opts)
	} else {
		printEntries(entries, opts)
	}
}

// printHeading prints a column header line.
// R2.1: -H/--heading prints header matching GNU who format.
// Type flags (-b, -u) add PID column; -u also adds IDLE column.
func printHeading(opts options) {
	if opts.showIdle {
		fmt.Println("NAME     LINE         TIME             IDLE          PID COMMENT")
	} else if opts.boot {
		fmt.Println("NAME     LINE         TIME                PID COMMENT")
	} else {
		fmt.Println("NAME     LINE         TIME         COMMENT")
	}
}

// printCount prints login names and a count of logged-in users.
// R2.4: -q/--count mode.
func printCount(entries []utmpxEntry) {
	var users []string
	for _, e := range entries {
		if e.entType == C.USER_PROCESS {
			users = append(users, e.user)
		}
	}
	if len(users) > 0 {
		fmt.Println(strings.Join(users, " "))
	}
	fmt.Printf("# users=%d\n", len(users))
}

// printEntries prints all collected utmpx entries.
func printEntries(entries []utmpxEntry, opts options) {
	for _, e := range entries {
		if e.entType == C.BOOT_TIME {
			printBootLine(e)
		} else {
			printUserLine(e, opts)
		}
	}
}

// printAmI prints only the entry matching the current terminal.
// R1.3: "who am i" prints the entry for the caller's terminal.
func printAmI(entries []utmpxEntry, opts options) {
	ttyName := currentTTY()
	for _, e := range entries {
		if e.line == ttyName {
			printUserLine(e, opts)
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

// printBootLine prints a boot time entry.
// R2.3: -b/--boot shows the time of the last system boot.
func printBootLine(e utmpxEntry) {
	timeStr := e.time.Format("Jan _2 15:04")
	fmt.Printf("         %-12s %s\n", "system boot", timeStr)
}

// printUserLine formats and prints a single user entry.
func printUserLine(e utmpxEntry, opts options) {
	timeStr := e.time.Format("Jan _2 15:04")
	if opts.showIdle {
		idle := getIdleStr(e.line)
		printUserWithIdle(e, timeStr, idle)
	} else {
		printUserBasic(e, timeStr)
	}
}

// printUserBasic prints a user line without idle time.
// R1.1: format matches GNU who default: "%-8s %-12s %s".
func printUserBasic(e utmpxEntry, timeStr string) {
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s (%s)\n", e.user, e.line, timeStr, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s\n", e.user, e.line, timeStr)
	}
}

// printUserWithIdle prints a user line with idle time and PID.
// R2.2: -u/--users shows idle time for each user.
func printUserWithIdle(e utmpxEntry, timeStr, idle string) {
	if e.host != "" {
		fmt.Printf("%-8s %-12s %s %s %5d (%s)\n",
			e.user, e.line, timeStr, idle, e.pid, e.host)
	} else {
		fmt.Printf("%-8s %-12s %s %s %5d\n",
			e.user, e.line, timeStr, idle, e.pid)
	}
}

// getIdleStr returns the idle time string for a terminal device.
// R2.2: idle is "." for active, "old" for >24h, "HH:MM" otherwise.
func getIdleStr(line string) string {
	devPath := "/dev/" + line
	info, err := sys.Stat(devPath)
	if err != nil {
		return "  ?  "
	}
	return formatIdle(time.Since(info.AccessTime))
}

// formatIdle converts a duration to GNU who idle format string.
func formatIdle(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return "  .  "
	}
	if secs >= 86400 {
		return " old "
	}
	hours := secs / 3600
	mins := (secs % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}
