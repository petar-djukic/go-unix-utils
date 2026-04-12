// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/pinky: lightweight finger information lookup.
// Implements srd098-pinky R1.1-R1.3: core utmpx reading and default output.
// Implements srd098-pinky R2.1-R2.3: flags and display options.
// Implements srd098-pinky R3.1-R3.3: error handling, version, and help.
package main

/*
#include <utmpx.h>
#include <pwd.h>
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

const progName = "pinky"

// timeFormat matches GNU pinky short-format login time: "Mon DD HH:MM".
const timeFormat = "Jan _2 15:04"

// utmpEntry holds one user session from the utmpx database.
type utmpEntry struct {
	user string
	line string
	time time.Time
	host string
}

// options holds parsed command-line flags.
type options struct {
	longFormat      bool // R2.1: -l
	suppressHeader  bool // R2.2: -f
	suppressHomeDir bool // R2.3: -b
	suppressProject bool // R2.3: -h
	suppressPlan    bool // R2.3: -p
	suppressName    bool // -w
	version         bool // R3.1: --version
	help            bool // R3.2: --help
}

// gecosFields holds parsed GECOS information.
// D1: real name, office, office phone, home phone.
type gecosFields struct {
	realName    string
	office      string
	officePhone string
	homePhone   string
}

// passwdInfo holds user information from the passwd database.
type passwdInfo struct {
	username string
	gecos    gecosFields
	homeDir  string
	shell    string
}

// main is the entry point for cmd/pinky.
func main() {
	sys.InstallSIGPIPEHandler()

	opts, operands, err := parseArgs(os.Args[1:])
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
	if opts.longFormat {
		os.Exit(runLongFormat(opts, operands))
		return
	}
	runShortFormat(opts, operands)
}

// runShortFormat displays logged-in users in short format.
func runShortFormat(opts options, operands []string) {
	entries := readUserEntries()
	if len(operands) > 0 {
		entries = filterByUsers(entries, operands)
	}
	if !opts.suppressHeader {
		printHeader(opts)
	}
	printShortEntries(entries, opts)
}

// runLongFormat displays long-format user information.
// R2.1: requires at least one username operand, matching GNU behavior.
func runLongFormat(opts options, operands []string) int {
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr,
			"%s: no username specified; at least one must be "+
				"specified when using -l\n"+
				"Try '%s --help' for more information.\n",
			progName, progName)
		return 1
	}
	for _, username := range operands {
		printLongEntry(username, opts)
	}
	return 0
}

// parseArgs parses command-line arguments into options and operands.
// Returns an error for unrecognized flags (R3.3).
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var operands []string
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := applyLongFlag(&opts, arg); err != nil {
				return options{}, nil, err
			}
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if err := parseShortFlags(&opts, arg); err != nil {
				return options{}, nil, err
			}
			continue
		}
		operands = append(operands, arg)
	}
	return opts, operands, nil
}

// applyLongFlag handles long-form flags (--version, --help).
func applyLongFlag(opts *options, arg string) error {
	switch arg {
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

// parseShortFlags processes a short flag argument, supporting combined flags.
func parseShortFlags(opts *options, arg string) error {
	for _, ch := range arg[1:] {
		if err := applyShortFlag(opts, ch); err != nil {
			return err
		}
	}
	return nil
}

// applyShortFlag sets the option for a single flag character.
func applyShortFlag(opts *options, ch rune) error {
	switch ch {
	case 'l':
		opts.longFormat = true
	case 's':
		// R1.3: short format is default, accepted silently.
	case 'f':
		opts.suppressHeader = true
	case 'b':
		opts.suppressHomeDir = true
	case 'h':
		opts.suppressProject = true
	case 'p':
		opts.suppressPlan = true
	case 'w':
		opts.suppressName = true
	default:
		return fmt.Errorf("%s: invalid option -- '%c'\nTry '%s --help' for more information.",
			progName, ch, progName)
	}
	return nil
}

// printVersion prints version information to stdout and exits 0.
// R3.1: matches GNU pinky --version output structure.
func printVersion() {
	fmt.Printf("%s (go-unix-utils) 0.1\n", progName)
}

// printUsage prints usage information to stdout.
// R3.2: matches GNU pinky --help output structure.
func printUsage() {
	fmt.Printf("Usage: %s [OPTION]... [USER]...\n", progName)
	fmt.Println()
	fmt.Println("Print information about users who are currently logged in.")
	fmt.Println()
	fmt.Println("  -l        produce long format output")
	fmt.Println("  -b        omit the user's home directory and shell in long format")
	fmt.Println("  -h        omit the user's project file in long format")
	fmt.Println("  -p        omit the user's plan file in long format")
	fmt.Println("  -s        do short format output, this is the default")
	fmt.Println("  -f        omit the line of column headings in short format")
	fmt.Println("  -w        omit the user's full name in short format")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
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

// filterByUsers returns entries whose login name matches one of the operands.
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
// R2.2: suppressed when -f is set.
// Column widths match GNU pinky: Login(9) Name(21) TTY(9) Idle(7) When(13) Where.
func printHeader(opts options) {
	if opts.suppressName {
		fmt.Printf("%-10s%-9s%-7s%-13s%s\n",
			"Login", "TTY", "Idle", "When", "Where")
		return
	}
	fmt.Printf("%-9s%-21s%-9s%-7s%-13s%s\n",
		"Login", "Name", "TTY", "Idle", "When", "Where")
}

// printShortEntries prints all user entries in short format.
func printShortEntries(entries []utmpEntry, opts options) {
	for _, e := range entries {
		printShortEntry(e, opts)
	}
}

// printShortEntry prints one user entry in short format.
// R1.1: shows login name, full name, tty, idle, login time, remote host.
func printShortEntry(e utmpEntry, opts options) {
	idle := idleString(e.line)
	timeStr := e.time.Format(timeFormat)
	if opts.suppressName {
		printShortLine("%-10s%-9s%-7s", e.user, e.line, idle, timeStr, e.host)
		return
	}
	fullName := lookupRealName(e.user)
	printShortLineWithName(e.user, fullName, e.line, idle, timeStr, e.host)
}

// printShortLine prints a short-format line without the name column.
func printShortLine(prefix string, user, line, idle, when, host string) {
	fmt.Printf(prefix, user, line, idle)
	printWhenHost(when, host)
}

// printShortLineWithName prints a short-format line with the name column.
func printShortLineWithName(user, name, line, idle, when, host string) {
	fmt.Printf("%-9s%-21s%-9s%-7s", user, name, line, idle)
	printWhenHost(when, host)
}

// printWhenHost prints the When and optional Where fields, avoiding trailing spaces.
func printWhenHost(when, host string) {
	if host != "" {
		fmt.Printf("%-13s%s\n", when, host)
	} else {
		fmt.Printf("%s\n", when)
	}
}

// lookupRealName returns the GECOS real name for the given username.
func lookupRealName(username string) string {
	pw, ok := lookupPasswd(username)
	if !ok {
		return "???"
	}
	return pw.gecos.realName
}

// lookupPasswd retrieves passwd entry via C getpwnam.
func lookupPasswd(username string) (passwdInfo, bool) {
	cName := C.CString(username)
	defer C.free(unsafe.Pointer(cName))
	pw := C.getpwnam(cName)
	if pw == nil {
		return passwdInfo{}, false
	}
	return passwdInfo{
		username: C.GoString(pw.pw_name),
		gecos:    parseGECOS(C.GoString(pw.pw_gecos)),
		homeDir:  C.GoString(pw.pw_dir),
		shell:    C.GoString(pw.pw_shell),
	}, true
}

// parseGECOS splits the GECOS field into its components.
// D1: comma-separated: real name, office, office phone, home phone.
func parseGECOS(gecos string) gecosFields {
	parts := strings.SplitN(gecos, ",", 4)
	var g gecosFields
	if len(parts) > 0 {
		g.realName = parts[0]
	}
	if len(parts) > 1 {
		g.office = parts[1]
	}
	if len(parts) > 2 {
		g.officePhone = parts[2]
	}
	if len(parts) > 3 {
		g.homePhone = parts[3]
	}
	return g
}

// printLongEntry prints long-format information for one user.
// R2.1: login name, real name, directory, shell, office, phone.
func printLongEntry(username string, opts options) {
	pw, ok := lookupPasswd(username)
	if !ok {
		printLongNameLine(username, "???")
		return
	}
	printLongNameLine(pw.username, pw.gecos.realName)
	if !opts.suppressHomeDir {
		printLongDirLine(pw.homeDir, pw.shell)
	}
	printLongContactLine(pw.gecos)
	// TODO: R2.3 .project/.plan display is a non-goal on macOS (srd098 non_goals).
	fmt.Println()
}

// printLongNameLine prints the login/real name line in long format.
func printLongNameLine(username, realName string) {
	fmt.Printf("Login name: %-28sIn real life:  %s\n",
		username, realName)
}

// printLongDirLine prints the directory/shell line in long format.
// R2.3: suppressed by -b flag.
func printLongDirLine(homeDir, shell string) {
	fmt.Printf("Directory: %-29sShell:  %s\n", homeDir, shell)
}

// printLongContactLine prints office/phone info if any fields are present.
func printLongContactLine(g gecosFields) {
	left := buildOfficeString(g.office, g.officePhone)
	if g.homePhone != "" {
		fmt.Printf("%-40sHome Phone: %s\n", left, g.homePhone)
	} else if left != "" {
		fmt.Println(left)
	}
}

// buildOfficeString constructs the office portion of the contact line.
func buildOfficeString(office, officePhone string) string {
	var parts []string
	if office != "" {
		parts = append(parts, office)
	}
	if officePhone != "" {
		parts = append(parts, officePhone)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Office: " + strings.Join(parts, ", ")
}

// idleString computes the idle time string for a terminal device.
// D2: computed from tty device modification time.
// Returns bare strings; column formatting is handled by the caller's printf.
func idleString(line string) string {
	devPath := "/dev/" + line
	info, err := os.Stat(devPath)
	if err != nil {
		return "?"
	}
	idle := time.Since(info.ModTime())
	if idle < time.Minute {
		return ""
	}
	const day = 24 * time.Hour
	if idle >= day {
		days := int(idle.Hours()) / 24
		return fmt.Sprintf("%dd", days)
	}
	hours := int(idle.Hours())
	minutes := int(idle.Minutes()) % 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
