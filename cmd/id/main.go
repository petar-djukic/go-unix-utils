// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd041-id R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: id [OPTION]... [USER]
Print user and group information for each specified USER,
or (when USER omitted) for the current user.

  -u, --user             print only the effective user ID
  -g, --group            print only the effective group ID
  -G, --groups           print all group IDs
  -n, --name             print a name instead of a number, for -ugG
  -r, --real             print the real ID instead of the effective ID, with -ug
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `id (go-unix-utils) dev
`

type flags struct {
	user   bool
	group  bool
	groups bool
	name   bool
	real   bool
}

type userInfo struct {
	uid      int
	gid      int
	username string
	groups   []int
}

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	remaining, f := parseFlags(args)
	checkConflictingFlags(f)
	checkModifierFlags(f)
	info := resolveUser(remaining)

	switch {
	case f.user:
		printUID(info, f)
	case f.group:
		printGID(info, f)
	case f.groups:
		printGroups(info, f)
	default:
		printDefault(info)
	}
}

func checkConflictingFlags(f flags) {
	count := 0
	if f.user {
		count++
	}
	if f.group {
		count++
	}
	if f.groups {
		count++
	}
	if count > 1 {
		fmt.Fprintln(os.Stderr, "id: cannot print \"only\" of more than one choice")
		os.Exit(1)
	}
}

func checkModifierFlags(f flags) {
	if (f.name || f.real) && !f.user && !f.group && !f.groups {
		fmt.Fprintln(os.Stderr, "id: printing only names or real IDs requires -u, -g, or -G")
		os.Exit(1)
	}
}

func resolveUser(remaining []string) userInfo {
	if len(remaining) == 0 {
		return currentUserInfo()
	}
	if len(remaining) > 1 {
		fmt.Fprintf(os.Stderr, "id: extra operand '%s'\n", remaining[1])
		fmt.Fprintln(os.Stderr, "Try 'id --help' for more information.")
		os.Exit(1)
	}
	return namedUserInfo(remaining[0])
}

func currentUserInfo() userInfo {
	uid := os.Getuid()
	gid := os.Getgid()
	gids, err := syscall.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: %v\n", err)
		os.Exit(1)
	}
	return userInfo{
		uid:      uid,
		gid:      gid,
		username: lookupUserName(uid),
		groups:   gids,
	}
}

func namedUserInfo(username string) userInfo {
	u, err := user.Lookup(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: '%s': no such user\n", username)
		os.Exit(1)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	groupStrs, err := u.GroupIds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: %v\n", err)
		os.Exit(1)
	}
	groups := make([]int, len(groupStrs))
	for i, s := range groupStrs {
		groups[i], _ = strconv.Atoi(s)
	}
	return userInfo{
		uid:      uid,
		gid:      gid,
		username: u.Username,
		groups:   groups,
	}
}

func printUID(info userInfo, f flags) {
	var out string
	if f.name {
		out = info.username
		if out == "" {
			out = strconv.Itoa(info.uid)
		}
	} else {
		out = strconv.Itoa(info.uid)
	}
	if _, err := fmt.Fprintln(os.Stdout, out); err != nil {
		os.Exit(1)
	}
}

func printGID(info userInfo, f flags) {
	var out string
	if f.name {
		out = lookupGroupName(info.gid)
		if out == "" {
			out = strconv.Itoa(info.gid)
		}
	} else {
		out = strconv.Itoa(info.gid)
	}
	if _, err := fmt.Fprintln(os.Stdout, out); err != nil {
		os.Exit(1)
	}
}

func printGroups(info userInfo, f flags) {
	parts := make([]string, len(info.groups))
	for i, g := range info.groups {
		if f.name {
			name := lookupGroupName(g)
			if name != "" {
				parts[i] = name
			} else {
				parts[i] = strconv.Itoa(g)
			}
		} else {
			parts[i] = strconv.Itoa(g)
		}
	}
	if _, err := fmt.Fprintln(os.Stdout, strings.Join(parts, " ")); err != nil {
		os.Exit(1)
	}
}

func printDefault(info userInfo) {
	uidEntry := formatID(info.uid, info.username)
	gidEntry := formatID(info.gid, lookupGroupName(info.gid))

	groupEntries := make([]string, 0, len(info.groups))
	for _, g := range info.groups {
		groupEntries = append(groupEntries, formatID(g, lookupGroupName(g)))
	}

	line := fmt.Sprintf("uid=%s gid=%s groups=%s",
		uidEntry, gidEntry, strings.Join(groupEntries, ","))
	if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
		os.Exit(1)
	}
}

func formatID(id int, name string) string {
	if name == "" {
		return strconv.Itoa(id)
	}
	return fmt.Sprintf("%d(%s)", id, name)
}

func lookupUserName(uid int) string {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return ""
	}
	return u.Username
}

func lookupGroupName(gid int) string {
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return ""
	}
	return g.Name
}

func parseFlags(args []string) ([]string, flags) {
	f := flags{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			return args[i+1:], f
		}
		if handled := parseLongFlag(arg, &f); handled {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "id: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'id --help' for more information.")
			os.Exit(1)
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			parseShortFlags(arg[1:], &f)
			i++
			continue
		}
		return args[i:], f
	}
	return nil, f
}

func parseLongFlag(arg string, f *flags) bool {
	switch arg {
	case "--user":
		f.user = true
	case "--group":
		f.group = true
	case "--groups":
		f.groups = true
	case "--name":
		f.name = true
	case "--real":
		f.real = true
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	default:
		return false
	}
	return true
}

func parseShortFlags(s string, f *flags) {
	for _, ch := range s {
		switch ch {
		case 'u':
			f.user = true
		case 'g':
			f.group = true
		case 'G':
			f.groups = true
		case 'n':
			f.name = true
		case 'r':
			f.real = true
		default:
			fmt.Fprintf(os.Stderr, "id: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'id --help' for more information.")
			os.Exit(1)
		}
	}
}
