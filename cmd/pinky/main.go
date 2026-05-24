// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd098-pinky R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
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

const helpText = `Usage: pinky [OPTION]... [USER]...

  -l     produce long format output for specified USERs
  -b     omit the user's home directory and shell in long format
  -f     omit the line of column headings in short format
  -h     omit the user's project file in long format
  -p     omit the user's plan file in long format
  -s     do short format output, this is the default
      --help     display this help and exit
      --version  output version information and exit

A lightweight 'finger' program;  print user information.
The utmp file will be /var/run/utmpx.
`

const versionText = `pinky (go-unix-utils) dev
`

var (
	longFormat     bool
	suppressHeader bool
	suppressHomeSh bool
)

type utmpEntry struct {
	user string
	line string
	host string
	time int64
}

type passwdInfo struct {
	name   string
	office string
	phone  string
	dir    string
	shell  string
}

func main() {
	sys.InstallSIGPIPEHandler()
	users := parseArgs(os.Args[1:])
	if longFormat {
		if len(users) == 0 {
			fmt.Fprintln(os.Stderr, "pinky: no username specified; at least one must be specified when using -l")
			fmt.Fprintln(os.Stderr, "Try 'pinky --help' for more information.")
			os.Exit(1)
		}
		if err := printLong(users); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := printShort(users); err != nil {
		os.Exit(1)
	}
}

func parseArgs(args []string) []string {
	var users []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			users = append(users, arg)
			continue
		}
		switch {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--":
			endOfFlags = true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "pinky: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'pinky --help' for more information.")
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			parseFlagBundle(arg[1:])
		default:
			users = append(users, arg)
		}
	}
	return users
}

func parseFlagBundle(flags string) {
	for _, ch := range flags {
		switch ch {
		case 'l':
			longFormat = true
		case 's':
			longFormat = false
		case 'f':
			suppressHeader = true
		case 'b':
			suppressHomeSh = true
		case 'h', 'p':
			// Accepted; no-op on macOS (no .project/.plan files).
		default:
			fmt.Fprintf(os.Stderr, "pinky: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'pinky --help' for more information.")
			os.Exit(1)
		}
	}
}

func printShort(users []string) error {
	entries := readEntries(users)
	if !suppressHeader {
		hdr := fmt.Sprintf("%-9s%-21s%-9s%-7s%-17s%s",
			"Login", "Name", "TTY", "Idle", "When", "Where")
		if _, err := fmt.Fprintln(os.Stdout, hdr); err != nil {
			return err
		}
	}
	for _, e := range entries {
		if err := printShortEntry(e); err != nil {
			return err
		}
	}
	return nil
}

func printShortEntry(e utmpEntry) error {
	name := lookupGecos(e.user)
	idle := idleString(e.line)
	when := formatLoginTime(e.time)
	var line string
	if e.host != "" {
		line = fmt.Sprintf("%-9s%-21s%-9s%-7s%-17s%s",
			e.user, name, e.line, idle, when, e.host)
	} else {
		line = fmt.Sprintf("%-9s%-21s%-9s%-7s%s",
			e.user, name, e.line, idle, when)
	}
	_, err := fmt.Fprintln(os.Stdout, line)
	return err
}

func printLong(users []string) error {
	for _, u := range users {
		if err := printLongEntry(u); err != nil {
			return err
		}
	}
	return nil
}

func printLongEntry(user string) error {
	pw := lookupPasswd(user)
	name := "???"
	if pw != nil {
		name = pw.name
	}
	if _, err := fmt.Fprintf(os.Stdout, "Login name: %-28sIn real life:  %s\n", user, name); err != nil {
		return err
	}
	if pw != nil && !suppressHomeSh {
		if _, err := fmt.Fprintf(os.Stdout, "Directory: %-29sShell:  %s\n", pw.dir, pw.shell); err != nil {
			return err
		}
	}
	if pw == nil {
		return nil
	}
	if err := printOffice(pw); err != nil {
		return err
	}
	_, err := fmt.Fprintln(os.Stdout)
	return err
}

func printOffice(pw *passwdInfo) error {
	if pw.office == "" && pw.phone == "" {
		return nil
	}
	var line string
	if pw.office != "" && pw.phone != "" {
		line = fmt.Sprintf("Office: %s, %s", pw.office, pw.phone)
	} else if pw.office != "" {
		line = fmt.Sprintf("Office: %s", pw.office)
	} else {
		line = fmt.Sprintf("Office: %s", pw.phone)
	}
	_, err := fmt.Fprintln(os.Stdout, line)
	return err
}

func readEntries(users []string) []utmpEntry {
	C.setutxent()
	defer C.endutxent()
	filter := make(map[string]bool, len(users))
	for _, u := range users {
		filter[u] = true
	}
	var entries []utmpEntry
	for {
		ent := C.getutxent()
		if ent == nil {
			break
		}
		if ent.ut_type != C.USER_PROCESS {
			continue
		}
		user := C.GoString(&ent.ut_user[0])
		if len(filter) > 0 && !filter[user] {
			continue
		}
		entries = append(entries, utmpEntry{
			user: user,
			line: C.GoString(&ent.ut_line[0]),
			host: C.GoString(&ent.ut_host[0]),
			time: int64(ent.ut_tv.tv_sec),
		})
	}
	return entries
}

func idleString(ttyName string) string {
	fi, err := sys.Stat("/dev/" + ttyName)
	if err != nil {
		return "?"
	}
	secs := int(time.Since(fi.AccessTime).Seconds())
	if secs < 60 {
		return ""
	}
	if secs < 24*60*60 {
		return fmt.Sprintf("%02d:%02d", secs/3600, (secs%3600)/60)
	}
	return fmt.Sprintf("%dd", secs/(24*60*60))
}

func formatLoginTime(sec int64) string {
	return time.Unix(sec, 0).Format("2006-01-02 15:04")
}

func lookupGecos(user string) string {
	cuser := C.CString(user)
	defer C.free(unsafe.Pointer(cuser))
	pw := C.getpwnam(cuser)
	if pw == nil {
		return "???"
	}
	gecos := C.GoString(pw.pw_gecos)
	name, _, _ := strings.Cut(gecos, ",")
	return name
}

func lookupPasswd(user string) *passwdInfo {
	cuser := C.CString(user)
	defer C.free(unsafe.Pointer(cuser))
	pw := C.getpwnam(cuser)
	if pw == nil {
		return nil
	}
	gecos := C.GoString(pw.pw_gecos)
	fields := strings.SplitN(gecos, ",", 4)
	info := &passwdInfo{
		dir:   C.GoString(pw.pw_dir),
		shell: C.GoString(pw.pw_shell),
	}
	if len(fields) > 0 {
		info.name = fields[0]
	}
	if len(fields) > 1 {
		info.office = fields[1]
	}
	if len(fields) > 2 {
		info.phone = fields[2]
	}
	return info
}
