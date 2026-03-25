// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd097-who: Show Who Is Logged On.
// Covers R1.1-R1.4 (utmpx reading, FILE argument, who am i, error handling),
// R2.1 (-H heading), R2.3 (-b boot time), R3.1-R3.3 (exit codes, SIGPIPE).
package main

/*
#include <utmpx.h>
#include <stdlib.h>
#include <unistd.h>
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

// whoEntry holds a single parsed utmpx record.
type whoEntry struct {
	user      string
	line      string
	loginTime time.Time
	host      string
	pid       int
	entryType int
}

// options holds parsed command-line flags.
type options struct {
	heading  bool
	boot     bool
	dead     bool
	login    bool
	process  bool
	runlevel bool
	timechg  bool
	showIdle bool
	count    bool
	all      bool
	amI      bool
	file     string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the who command logic and returns the exit code.
func run(args []string) int {
	opts, code := parseArgs(args)
	if code >= 0 {
		return code
	}
	if opts.file != "" {
		if err := setUtmpxFile(opts.file); err != nil {
			fmt.Fprintf(os.Stderr, "who: %v\n", err)
			return 1
		}
	}
	entries, err := readEntries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "who: %v\n", err)
		return 1
	}
	filtered := filterEntries(entries, opts)
	return printOutput(filtered, opts)
}

// parseArgs parses command-line arguments into options.
// Returns (opts, -1) on success or (opts, exitCode) to exit.
func parseArgs(args []string) (options, int) {
	var opts options
	var positional []string
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
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
		positional = append(positional, arg)
	}
	return resolvePositional(positional, opts)
}

// resolvePositional handles positional arguments (FILE or am-i mode).
func resolvePositional(pos []string, opts options) (options, int) {
	switch len(pos) {
	case 0:
		// default: no file, no am-i
	case 1:
		opts.file = pos[0]
	case 2:
		// R1.3: "who am i" triggered by two positional args.
		opts.amI = true
	default:
		fmt.Fprintf(os.Stderr, "who: extra operand '%s'\n", pos[2])
		return opts, 1
	}
	if opts.all {
		applyAllFlag(&opts)
	}
	return opts, -1
}

// applyAllFlag sets all type filter flags for -a/--all.
func applyAllFlag(opts *options) {
	opts.boot = true
	opts.dead = true
	opts.login = true
	opts.process = true
	opts.runlevel = true
	opts.timechg = true
	opts.showIdle = true
}

// applyLongFlag handles a single --flag. Returns -1 on success.
func applyLongFlag(name string, opts *options) int {
	switch name {
	case "all":
		opts.all = true
	case "boot":
		opts.boot = true
	case "dead":
		opts.dead = true
	case "heading":
		opts.heading = true
	case "login":
		opts.login = true
	case "process":
		opts.process = true
	case "runlevel":
		opts.runlevel = true
	case "time":
		opts.timechg = true
	case "users":
		opts.showIdle = true
	case "count":
		opts.count = true
	case "short":
		// default format, ignored
	case "mesg", "message", "writable":
		// TODO: message status not in prd097 scope
	case "lookup":
		// TODO: DNS lookup not in prd097 scope
	case "help":
		return printHelp()
	case "version":
		return printVersion()
	default:
		fmt.Fprintf(os.Stderr,
			"who: unrecognized option '--%s'\n", name)
		return 1
	}
	return -1
}

// applyShortFlags handles combined short flags like -bH.
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
	case 'a':
		opts.all = true
	case 'b':
		opts.boot = true
	case 'd':
		opts.dead = true
	case 'H':
		opts.heading = true
	case 'l':
		opts.login = true
	case 'm':
		opts.amI = true
	case 'p':
		opts.process = true
	case 'q':
		opts.count = true
	case 'r':
		opts.runlevel = true
	case 's':
		// short format is default, ignored
	case 't':
		opts.timechg = true
	case 'T', 'w':
		// TODO: message status not in prd097 scope
	case 'u':
		opts.showIdle = true
	default:
		fmt.Fprintf(os.Stderr,
			"who: invalid option -- '%c'\n", ch)
		return 1
	}
	return -1
}

// readEntries reads all utmpx entries from the system database.
// R1.1: read utmpx database.
func readEntries() ([]whoEntry, error) {
	C.setutxent()
	defer C.endutxent()

	var entries []whoEntry
	for {
		entry := C.getutxent()
		if entry == nil {
			break
		}
		entries = append(entries, parseUtmpxEntry(entry))
	}
	return entries, nil
}

// parseUtmpxEntry converts a C utmpx struct to a Go whoEntry.
func parseUtmpxEntry(entry *C.struct_utmpx) whoEntry {
	return whoEntry{
		user:      C.GoString(&entry.ut_user[0]),
		line:      C.GoString(&entry.ut_line[0]),
		loginTime: time.Unix(int64(entry.ut_tv.tv_sec), 0),
		host:      C.GoString(&entry.ut_host[0]),
		pid:       int(entry.ut_pid),
		entryType: int(entry.ut_type),
	}
}

// setUtmpxFile configures utmpx to read from a specific file.
// R1.2: accept optional FILE argument.
func setUtmpxFile(file string) error {
	if _, err := os.Stat(file); err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return fmt.Errorf("%s: %v", pe.Path, pe.Err)
		}
		return err
	}
	cfile := C.CString(file)
	defer C.free(unsafe.Pointer(cfile))
	C.utmpxname(cfile)
	return nil
}

// hasTypeFilter returns true if any entry-type filter flag is active.
func hasTypeFilter(opts options) bool {
	return opts.boot || opts.dead || opts.login ||
		opts.process || opts.runlevel || opts.timechg || opts.showIdle
}

// filterEntries selects entries matching the active type filters.
func filterEntries(entries []whoEntry, opts options) []whoEntry {
	if opts.amI {
		return filterCurrentTTY(entries)
	}
	if opts.count || !hasTypeFilter(opts) {
		return filterByType(entries, int(C.USER_PROCESS))
	}
	var result []whoEntry
	for i := range entries {
		if matchesFilter(entries[i], opts) {
			result = append(result, entries[i])
		}
	}
	return result
}

// filterCurrentTTY returns only the entry for the current terminal.
// R1.3: "who am i" prints the entry for the current terminal.
func filterCurrentTTY(entries []whoEntry) []whoEntry {
	ttyLine := currentTTYLine()
	if ttyLine == "" {
		return nil
	}
	for i := range entries {
		if entries[i].entryType == int(C.USER_PROCESS) &&
			entries[i].line == ttyLine {
			return []whoEntry{entries[i]}
		}
	}
	return nil
}

// filterByType returns entries matching a specific utmpx type.
func filterByType(entries []whoEntry, entryType int) []whoEntry {
	var result []whoEntry
	for i := range entries {
		if entries[i].entryType == entryType {
			result = append(result, entries[i])
		}
	}
	return result
}

// matchesFilter checks if an entry matches any active type filter.
func matchesFilter(e whoEntry, opts options) bool {
	t := e.entryType
	switch {
	case opts.boot && t == int(C.BOOT_TIME):
		return true
	case opts.dead && t == int(C.DEAD_PROCESS):
		return true
	case opts.login && t == int(C.LOGIN_PROCESS):
		return true
	case opts.process && t == int(C.INIT_PROCESS):
		return true
	case opts.runlevel && t == int(C.RUN_LVL):
		return true
	case opts.timechg && t == int(C.NEW_TIME):
		return true
	case opts.showIdle && t == int(C.USER_PROCESS):
		return true
	}
	return false
}

// printOutput formats and prints filtered entries. Returns exit code.
func printOutput(entries []whoEntry, opts options) int {
	if opts.count {
		return printCountMode(entries)
	}
	if opts.heading {
		printHeading(opts)
	}
	for i := range entries {
		line := formatEntry(entries[i], opts)
		if _, err := fmt.Println(line); err != nil {
			return 1
		}
	}
	return 0
}

// printCountMode prints login names and a user count. R2.4.
func printCountMode(entries []whoEntry) int {
	names := make([]string, 0, len(entries))
	for i := range entries {
		names = append(names, entries[i].user)
	}
	if len(names) > 0 {
		fmt.Println(strings.Join(names, " "))
	}
	fmt.Printf("# users=%d\n", len(names))
	return 0
}

// printHeading prints the column header line. R2.1.
func printHeading(opts options) {
	if opts.showIdle {
		fmt.Printf("%-8s %-12s %-16s %-6s %5s %s\n",
			"NAME", "LINE", "TIME", "IDLE", "PID", "COMMENT")
	} else {
		fmt.Printf("%-8s %-12s %-16s %s\n",
			"NAME", "LINE", "TIME", "COMMENT")
	}
}

// formatEntry formats a single whoEntry as a display line.
func formatEntry(e whoEntry, opts options) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s %-12s %s",
		entryLabel(e), displayLine(e), formatTime(e.loginTime))
	if opts.showIdle && e.entryType == int(C.USER_PROCESS) {
		appendIdleAndPID(&b, e)
	}
	if e.host != "" {
		fmt.Fprintf(&b, " (%s)", e.host)
	}
	return b.String()
}

// entryLabel returns the user field for display.
func entryLabel(e whoEntry) string {
	switch e.entryType {
	case int(C.BOOT_TIME), int(C.NEW_TIME):
		return ""
	default:
		return e.user
	}
}

// displayLine returns the line field for display, translating
// special entry types to human-readable labels.
func displayLine(e whoEntry) string {
	switch e.entryType {
	case int(C.BOOT_TIME):
		return "system boot"
	case int(C.RUN_LVL):
		return "run-level"
	case int(C.NEW_TIME):
		return "clock change"
	default:
		return e.line
	}
}

// appendIdleAndPID adds idle time and PID to the output.
// R2.2: idle time display.
func appendIdleAndPID(b *strings.Builder, e whoEntry) {
	idle := computeIdleTime(e.line)
	fmt.Fprintf(b, "  %-6s %5d", idle, e.pid)
}

// computeIdleTime returns idle time for a terminal as a display string.
// Returns "." for active, "old" for > 24h, or "HH:MM".
func computeIdleTime(line string) string {
	devPath := "/dev/" + line
	info, err := os.Stat(devPath)
	if err != nil {
		return "?"
	}
	idle := time.Since(info.ModTime())
	if idle < time.Minute {
		return "."
	}
	if idle > 24*time.Hour {
		return "old"
	}
	hours := int(idle.Hours())
	minutes := int(idle.Minutes()) % 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// formatTime formats a time value in GNU who format (YYYY-MM-DD HH:MM).
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// currentTTYLine returns the terminal line name stripped of /dev/ prefix.
func currentTTYLine() string {
	tty := C.ttyname(0)
	if tty == nil {
		return ""
	}
	name := C.GoString(tty)
	return strings.TrimPrefix(name, "/dev/")
}

// printHelp writes usage information and returns the exit code.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: who [OPTION]... [ FILE | ARG1 ARG2 ]
Print information about users who are currently logged in.

  -a, --all         same as -b -d --login -p -r -t -T -u
  -b, --boot        time of last system boot
  -d, --dead        print dead processes
  -H, --heading     print line of column headings
  -l, --login       print system login processes
  -m                only hostname and user associated with stdin
  -p, --process     print active processes spawned by init
  -q, --count       all login names and number of users logged on
  -r, --runlevel    print current runlevel
  -s, --short       print only name, line, and time (default)
  -t, --time        print last system clock change
  -T, -w, --mesg    add user's message status as +, - or ?
  -u, --users       list users logged in
      --help        display this help and exit
      --version     output version information and exit
`)
	return 0
}

// printVersion writes version information and returns the exit code.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "who (go-unix-utils) %s\n", version)
	return 0
}
