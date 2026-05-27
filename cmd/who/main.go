// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd097-who R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
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

  -b, --boot     time of last system boot
  -H, --heading  print line of column headings
  -q, --count    all login names and number of users logged on
  -u, --users    list users logged in
      --help     display this help and exit
      --version  output version information and exit

If FILE is not specified, use /var/run/utmpx.  /var/log/wtmp as FILE is common.
If ARG1 ARG2 given, -m presumed: 'am i' or 'mom likes' are usual.
`

const versionText = `who (go-unix-utils) dev
`

var (
	showHeading bool
	showIdle    bool
	showBoot    bool
	showCount   bool
)

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

	if showCount {
		if err := printCount(amI); err != nil {
			os.Exit(1)
		}
		return
	}

	printUsers := showIdle || !showBoot
	if showHeading {
		if err := printHeader(printUsers); err != nil {
			os.Exit(1)
		}
	}
	if showBoot {
		if err := printBootEntry(); err != nil {
			os.Exit(1)
		}
	}
	if printUsers {
		entries := readEntries(amI)
		for _, e := range entries {
			if err := printEntry(e); err != nil {
				os.Exit(1)
			}
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

func readBootTime() (time.Time, bool) {
	C.setutxent()
	defer C.endutxent()
	var bootTime time.Time
	found := false
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		if entry.ut_type != C.BOOT_TIME {
			continue
		}
		bootTime = time.Unix(int64(entry.ut_tv.tv_sec), 0)
		found = true
	}
	return bootTime, found
}

func currentLine() string {
	tty := C.ttyname(C.int(os.Stdin.Fd()))
	if tty == nil {
		return ""
	}
	return strings.TrimPrefix(C.GoString(tty), "/dev/")
}

func printHeader(users bool) error {
	var hdr string
	if users && showIdle {
		hdr = fmt.Sprintf("%-8s %-12s %-16s %-6s%s", "NAME", "LINE", "TIME", "IDLE", "COMMENT")
	} else {
		hdr = fmt.Sprintf("%-8s %-12s %-16s %s", "NAME", "LINE", "TIME", "COMMENT")
	}
	_, err := fmt.Fprintln(os.Stdout, hdr)
	return err
}

func printEntry(e whoEntry) error {
	timeStr := e.time.Format("2006-01-02 15:04")
	if showIdle {
		return printEntryWithIdle(e, timeStr)
	}
	var line string
	if e.host != "" {
		line = fmt.Sprintf("%-8s %-12s %s (%s)", e.user, e.line, timeStr, e.host)
	} else {
		line = fmt.Sprintf("%-8s %-12s %s", e.user, e.line, timeStr)
	}
	_, err := fmt.Fprintln(os.Stdout, line)
	return err
}

func printEntryWithIdle(e whoEntry, timeStr string) error {
	idle := idleString(e.line)
	var line string
	if e.host != "" {
		line = fmt.Sprintf("%-8s %-12s %s %5s (%s)", e.user, e.line, timeStr, idle, e.host)
	} else {
		line = fmt.Sprintf("%-8s %-12s %s %5s", e.user, e.line, timeStr, idle)
	}
	_, err := fmt.Fprintln(os.Stdout, line)
	return err
}

func idleString(ttyName string) string {
	fi, err := sys.Stat("/dev/" + ttyName)
	if err != nil {
		return "?"
	}
	secs := int(time.Since(fi.AccessTime).Seconds())
	if secs < 60 {
		return "."
	}
	if secs >= 24*60*60 {
		return "old"
	}
	return fmt.Sprintf("%02d:%02d", secs/3600, (secs%3600)/60)
}

func printBootEntry() error {
	bootTime, found := readBootTime()
	if !found {
		return nil
	}
	timeStr := bootTime.Format("2006-01-02 15:04")
	line := fmt.Sprintf("%-8s %-12s %s", "", "system boot", timeStr)
	_, err := fmt.Fprintln(os.Stdout, line)
	return err
}

func printCount(amI bool) error {
	entries := readEntries(amI)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.user
	}
	if _, err := fmt.Fprintln(os.Stdout, strings.Join(names, " ")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(os.Stdout, "# users=%d\n", len(entries))
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
		case arg == "--heading":
			showHeading = true
		case arg == "--users":
			showIdle = true
		case arg == "--boot":
			showBoot = true
		case arg == "--count":
			showCount = true
		case arg == "--":
			endOfFlags = true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "who: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'who --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			parseShortFlags(arg[1:])
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

func parseShortFlags(flags string) {
	for _, ch := range flags {
		switch ch {
		case 'H':
			showHeading = true
		case 'u':
			showIdle = true
		case 'b':
			showBoot = true
		case 'q':
			showCount = true
		default:
			fmt.Fprintf(os.Stderr, "who: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'who --help' for more information.")
			os.Exit(1)
		}
	}
}
