// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd098-pinky: Lightweight Finger Information Lookup.
// Covers R1.1 (short format default), R1.2 (USER argument filtering),
// R1.3 (-s short format flag).
package main

/*
#include <utmpx.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// utmpEntry holds a single parsed utmpx record.
type utmpEntry struct {
	user      string
	line      string
	loginTime time.Time
	host      string
}

// options holds parsed command-line flags.
type options struct {
	longFmt   bool
	shortFmt  bool
	showName  bool
	showHost  bool
	showIdle  bool
	showHead  bool
	omitHome  bool
	omitProj  bool
	omitPlan  bool
	users     []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the pinky command logic and returns the exit code.
func run(args []string) int {
	opts, code := parseArgs(args)
	if code >= 0 {
		return code
	}
	if opts.longFmt && len(opts.users) == 0 {
		fmt.Fprintf(os.Stderr,
			"pinky: no username specified; at least one must be"+
				" specified when using -l\n")
		fmt.Fprintf(os.Stderr,
			"Try 'pinky --help' for more information.\n")
		return 1
	}
	return execute(opts)
}

// execute runs the appropriate output mode(s).
func execute(opts options) int {
	if opts.shortFmt {
		if code := shortPinky(opts); code != 0 {
			return code
		}
	}
	if opts.longFmt {
		for _, u := range opts.users {
			if code := longPinky(u, opts); code != 0 {
				return code
			}
		}
	}
	return 0
}

// defaultOptions returns options with default settings.
func defaultOptions() options {
	return options{
		shortFmt: true,
		showName: true,
		showHost: true,
		showIdle: true,
		showHead: true,
	}
}

// parseArgs parses command-line arguments into options.
// Returns (opts, -1) on success or (opts, exitCode) to exit.
func parseArgs(args []string) (options, int) {
	opts := defaultOptions()
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			opts.users = append(opts.users, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			if code := applyLongFlag(arg[2:], &opts); code >= 0 {
				return opts, code
			}
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			if code := applyShortFlags(arg[1:], &opts); code >= 0 {
				return opts, code
			}
			continue
		}
		opts.users = append(opts.users, arg)
	}
	return opts, -1
}

// applyLongFlag handles a single --flag. Returns -1 on success.
func applyLongFlag(name string, opts *options) int {
	switch name {
	case "lookup":
		// DNS lookup not in scope
	case "help":
		return printHelp()
	case "version":
		return printVersion()
	default:
		fmt.Fprintf(os.Stderr,
			"pinky: unrecognized option '--%s'\n", name)
		fmt.Fprintf(os.Stderr,
			"Try 'pinky --help' for more information.\n")
		return 1
	}
	return -1
}

// applyShortFlags handles combined short flags like -sf.
func applyShortFlags(flags string, opts *options) int {
	for _, ch := range flags {
		if code := applyShortFlag(ch, opts); code >= 0 {
			return code
		}
	}
	return -1
}

// applyShortFlag handles a single short flag character.
func applyShortFlag(ch rune, opts *options) int {
	switch ch {
	case 'l':
		opts.longFmt = true
		opts.shortFmt = false
	case 's':
		opts.shortFmt = true
		opts.longFmt = false
	case 'f':
		opts.showHead = false
	case 'w':
		opts.showName = false
	case 'i':
		opts.showName = false
		opts.showHost = false
	case 'q':
		opts.showName = false
		opts.showHost = false
		opts.showIdle = false
	case 'b':
		opts.omitHome = true
	case 'h':
		opts.omitProj = true
	case 'p':
		opts.omitPlan = true
	default:
		fmt.Fprintf(os.Stderr,
			"pinky: invalid option -- '%c'\n", ch)
		fmt.Fprintf(os.Stderr,
			"Try 'pinky --help' for more information.\n")
		return 1
	}
	return -1
}

// readEntries reads all USER_PROCESS utmpx entries.
func readEntries() []utmpEntry {
	C.setutxent()
	defer C.endutxent()

	var entries []utmpEntry
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		if int(entry.ut_type) != int(C.USER_PROCESS) {
			continue
		}
		entries = append(entries, parseUtmpxEntry(entry))
	}
	return entries
}

// parseUtmpxEntry converts a C utmpx struct to a Go utmpEntry.
func parseUtmpxEntry(entry *C.struct_utmpx) utmpEntry {
	return utmpEntry{
		user:      C.GoString(&entry.ut_user[0]),
		line:      C.GoString(&entry.ut_line[0]),
		loginTime: time.Unix(int64(entry.ut_tv.tv_sec), 0),
		host:      C.GoString(&entry.ut_host[0]),
	}
}

// shortPinky prints the short format output.
func shortPinky(opts options) int {
	entries := readEntries()
	if len(opts.users) > 0 {
		entries = filterByUsers(entries, opts.users)
	}
	if opts.showHead {
		printShortHeader(opts)
	}
	for i := range entries {
		if err := printShortEntry(entries[i], opts); err != nil {
			return 1
		}
	}
	return 0
}

// filterByUsers returns entries matching any of the given usernames.
func filterByUsers(entries []utmpEntry, users []string) []utmpEntry {
	set := make(map[string]bool, len(users))
	for _, u := range users {
		set[u] = true
	}
	var result []utmpEntry
	for i := range entries {
		if set[entries[i].user] {
			result = append(result, entries[i])
		}
	}
	return result
}

// printShortHeader prints the column header line.
func printShortHeader(opts options) {
	var b strings.Builder
	if opts.showName {
		fmt.Fprintf(&b, "%-8s", "Login")
	} else {
		fmt.Fprintf(&b, "%-9s", "Login")
	}
	if opts.showName {
		fmt.Fprintf(&b, " %-20s", "Name")
	}
	fmt.Fprintf(&b, " %-8s", "TTY")
	if opts.showIdle {
		fmt.Fprintf(&b, " %-6s", "Idle")
	}
	fmt.Fprintf(&b, " %-16s", "When")
	if opts.showHost {
		fmt.Fprintf(&b, " %s", "Where")
	}
	fmt.Println(b.String())
}

// printShortEntry prints a single short format entry.
func printShortEntry(e utmpEntry, opts options) error {
	var b strings.Builder
	if opts.showName {
		fmt.Fprintf(&b, "%-8s", e.user)
	} else {
		fmt.Fprintf(&b, "%-9s", e.user)
	}
	if opts.showName {
		name := lookupFullName(e.user)
		fmt.Fprintf(&b, " %-20s", name)
	}
	fmt.Fprintf(&b, " %-8s", e.line)
	if opts.showIdle {
		idle := idleString(e.line)
		fmt.Fprintf(&b, " %-6s", idle)
	}
	fmt.Fprintf(&b, " %s", formatTime(e.loginTime))
	if opts.showHost && e.host != "" {
		fmt.Fprintf(&b, " %s", e.host)
	}
	_, err := fmt.Println(b.String())
	return err
}

// lookupFullName returns the GECOS full name for a username.
func lookupFullName(username string) string {
	cname := C.CString(username)
	defer C.free(unsafe.Pointer(cname))
	pw := C.getpwnam(cname)
	if pw == nil {
		return " ???"
	}
	gecos := C.GoString(pw.pw_gecos)
	return extractName(gecos, username)
}

// extractName returns the full name from a GECOS string.
// Handles '&' replacement with capitalized login name.
func extractName(gecos, login string) string {
	// GECOS format: name,office,work-phone,home-phone
	name, _, _ := strings.Cut(gecos, ",")
	if strings.Contains(name, "&") {
		cap := strings.ToUpper(login[:1]) + login[1:]
		name = strings.ReplaceAll(name, "&", cap)
	}
	return name
}

// idleString returns the idle time display string for a terminal.
// Uses access time (atime) to match GNU pinky behavior.
func idleString(line string) string {
	devPath := "/dev/" + line
	info, err := sys.Stat(devPath)
	if err != nil {
		return " ?"
	}
	idle := time.Since(info.AccessTime)
	if idle < time.Minute {
		return "     "
	}
	totalSec := int(idle.Seconds())
	hours := totalSec / 3600
	if hours >= 24 {
		days := hours / 24
		return fmt.Sprintf("%dd", days)
	}
	minutes := (totalSec % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// formatTime formats a time value for pinky display (YYYY-MM-DD HH:MM).
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// longPinky prints long format output for a single user.
func longPinky(username string, opts options) int {
	cname := C.CString(username)
	defer C.free(unsafe.Pointer(cname))
	pw := C.getpwnam(cname)

	name := "???"
	dir := ""
	shell := ""
	if pw != nil {
		gecos := C.GoString(pw.pw_gecos)
		name = extractName(gecos, username)
		dir = C.GoString(pw.pw_dir)
		shell = C.GoString(pw.pw_shell)
	}
	if err := printLongName(username, name); err != nil {
		return 1
	}
	if pw == nil {
		return 0
	}
	if !opts.omitHome {
		if err := printLongDirShell(dir, shell); err != nil {
			return 1
		}
	}
	// R1.3/non_goals: .plan and .project not read on macOS
	if _, err := fmt.Println(); err != nil {
		return 1
	}
	return 0
}

// printLongName prints the login name and real name line.
func printLongName(username, fullname string) error {
	_, err := fmt.Printf("Login name: %-28sIn real life:  %s\n",
		username, fullname)
	return err
}

// printLongDirShell prints the directory and shell line.
func printLongDirShell(dir, shell string) error {
	_, err := fmt.Printf("Directory: %-29sShell:  %s\n", dir, shell)
	return err
}

// printHelp writes usage information and returns the exit code.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: pinky [OPTION]... [USER]...

  -l     produce long format output for the specified USERs
  -b     omit the user's home directory and shell in long format
  -h     omit the user's project file in long format
  -p     omit the user's plan file in long format
  -s     do short format output, this is the default
  -f     omit the line of column headings in short format
  -w     omit the user's full name in short format
  -i     omit the user's full name and remote host in short format
  -q     omit the user's full name, remote host and idle time
             in short format
      --help     display this help and exit
      --version  output version information and exit
`)
	return 0
}

// printVersion writes version information and returns the exit code.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "pinky (go-unix-utils) %s\n", version)
	return 0
}
