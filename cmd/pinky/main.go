// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pinky implements prd098-pinky R1.1-R1.3, R2.1-R2.3, R3.1-R3.3:
// lightweight finger information lookup showing logged-in user details
// in short or long format, verified against gpinky via differential testing.

package main

/*
#include <utmpx.h>
#include <pwd.h>
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
const progName = "pinky"

// timeStringLen is the expected width of formatted time strings with LC_ALL=C.
const timeStringLen = 12

// options holds parsed command-line flags and user operands.
type options struct {
	shortFormat     bool
	longFormat      bool
	suppressHeader  bool // R2.2: -f suppresses header in short format.
	suppressDir     bool // R2.3: -b suppresses home dir and shell in long format.
	suppressProject bool // R2.3: -h suppresses project file (no-op per non_goals).
	suppressPlan    bool // R2.3: -p suppresses plan file (no-op per non_goals).
	users           []string
}

// utmpxEntry holds fields extracted from a utmpx record.
type utmpxEntry struct {
	user string
	line string
	time time.Time
	host string
}

// passwdInfo holds fields extracted from a passwd entry.
type passwdInfo struct {
	name  string
	gecos string
	dir   string
	shell string
}

func main() {
	// R3.3: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
}

// run parses arguments and prints pinky output.
func run(args []string) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.longFormat && len(opts.users) == 0 {
		return fmt.Errorf("no username specified; at least one must be specified when using -l")
	}
	if opts.shortFormat {
		printShort(opts)
	}
	if opts.longFormat {
		printLong(opts)
	}
	return nil
}

// parseArgs extracts options and user operands from arguments.
// R1.2: user operands filter output to named users.
// R1.3: -s forces short format.
func parseArgs(args []string) (options, error) {
	opts := options{shortFormat: true}
	for i, arg := range args {
		if arg == "--" {
			opts.users = append(opts.users, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if err := applyFlags(&opts, arg[1:]); err != nil {
				return opts, err
			}
			continue
		}
		opts.users = append(opts.users, arg)
	}
	return opts, nil
}

// applyFlags processes combined short flags like -sl.
// R2.2: -f suppresses header. R2.3: -b, -h, -p flags.
func applyFlags(opts *options, flags string) error {
	for _, ch := range flags {
		switch ch {
		case 's':
			opts.shortFormat = true
		case 'l':
			opts.shortFormat = false
			opts.longFormat = true
		case 'f':
			opts.suppressHeader = true
		case 'b':
			opts.suppressDir = true
		case 'h':
			opts.suppressProject = true
		case 'p':
			opts.suppressPlan = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// readUtmpxEntries reads USER_PROCESS entries from the utmpx database.
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

// lookupPasswd returns passwd information for a username, or nil if not found.
func lookupPasswd(username string) *passwdInfo {
	cname := C.CString(username)
	defer C.free(unsafe.Pointer(cname))
	pw := C.getpwnam(cname)
	if pw == nil {
		return nil
	}
	return &passwdInfo{
		name:  C.GoString(pw.pw_name),
		gecos: C.GoString(pw.pw_gecos),
		dir:   C.GoString(pw.pw_dir),
		shell: C.GoString(pw.pw_shell),
	}
}

// gecosField extracts the field at index from a comma-separated GECOS string.
// Returns empty string if the index is out of range.
func gecosField(gecos string, index int) string {
	fields := strings.Split(gecos, ",")
	if index < len(fields) {
		return fields[index]
	}
	return ""
}

// gecosName extracts the first field (real name) from a GECOS string.
func gecosName(gecos string) string {
	return gecosField(gecos, 0)
}

// printShort prints logged-in users in short format.
// R1.1: default output shows login name, full name, terminal, idle, time, host.
// R2.2: -f suppresses the header line.
func printShort(opts options) {
	if !opts.suppressHeader {
		printShortHeader()
	}
	entries := readUtmpxEntries()
	userSet := makeUserSet(opts.users)
	for _, e := range entries {
		if len(userSet) > 0 && !userSet[e.user] {
			continue
		}
		printShortEntry(e)
	}
}

// makeUserSet creates a lookup set from user operands.
func makeUserSet(users []string) map[string]bool {
	if len(users) == 0 {
		return nil
	}
	m := make(map[string]bool, len(users))
	for _, u := range users {
		m[u] = true
	}
	return m
}

// printShortHeader prints the column header for short format.
func printShortHeader() {
	fmt.Printf("%-8s %-19s %-9s %-6s %-*s %s\n",
		"Login", "Name", " TTY", "Idle", timeStringLen, "When", "Where")
}

// printShortEntry prints one short-format line for a utmpx entry.
func printShortEntry(e utmpxEntry) {
	pw := lookupPasswd(e.user)
	idle := getIdleStr(e.line)
	timeStr := e.time.Format("Jan _2 15:04")
	fmt.Printf("%-8s", e.user)
	printShortName(pw)
	fmt.Printf("  %-8.8s %-6s %s", e.line, idle, timeStr)
	if e.host != "" {
		fmt.Printf(" %s", e.host)
	}
	fmt.Println()
}

// printShortName prints the GECOS name column in short format.
func printShortName(pw *passwdInfo) {
	if pw != nil {
		fmt.Printf(" %-19.19s", gecosName(pw.gecos))
	} else {
		fmt.Printf(" %19s", " ???")
	}
}

// printLong prints long-format user information.
// R2.1: shows login name, real name, directory, shell, office, phone.
// R2.3: -b suppresses directory and shell.
func printLong(opts options) {
	for _, user := range opts.users {
		printLongEntry(user, opts)
	}
}

// printLongEntry prints long-format information for one user.
// R2.3: -b suppresses directory and shell lines.
// R3.1: nonexistent users still exit 0; trailing blank line only when pw exists.
func printLongEntry(username string, opts options) {
	pw := lookupPasswd(username)
	printLongNameLine(username, pw)
	if pw == nil {
		return
	}
	if !opts.suppressDir {
		fmt.Printf("Directory: %-29s", pw.dir)
		fmt.Printf("Shell:  %s\n", pw.shell)
	}
	printLongGecos(pw.gecos)
	fmt.Println()
}

// printLongNameLine prints the login/real-name line in long format.
func printLongNameLine(username string, pw *passwdInfo) {
	fmt.Printf("Login name: %-28s", username)
	fmt.Printf("In real life:  ")
	if pw != nil {
		fmt.Printf("%s", gecosName(pw.gecos))
	} else {
		fmt.Printf("???")
	}
	fmt.Println()
}

// printLongGecos prints office and phone fields from GECOS in long format.
func printLongGecos(gecos string) {
	office := gecosField(gecos, 1)
	officePhone := gecosField(gecos, 2)
	homePhone := gecosField(gecos, 3)
	printOfficeLine(office, officePhone)
	if homePhone != "" {
		fmt.Printf("Home Phone: %s\n", homePhone)
	}
}

// printOfficeLine prints the office location and phone if present.
func printOfficeLine(office, phone string) {
	if office == "" && phone == "" {
		return
	}
	if office != "" && phone != "" {
		fmt.Printf("Office: %s, %s\n", office, phone)
	} else if office != "" {
		fmt.Printf("Office: %s\n", office)
	} else {
		fmt.Printf("Office: %s\n", phone)
	}
}

// getIdleStr returns the idle time string for a terminal device.
func getIdleStr(line string) string {
	devPath := "/dev/" + line
	info, err := sys.Stat(devPath)
	if err != nil {
		return "?"
	}
	return formatIdle(time.Since(info.AccessTime))
}

// formatIdle converts a duration to pinky idle format.
// Active (<60s): empty, days (>=24h): "Nd", otherwise "HH:MM".
func formatIdle(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return ""
	}
	if secs >= 86400 {
		days := secs / 86400
		return fmt.Sprintf("%dd", days)
	}
	hours := secs / 3600
	mins := (secs % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}
