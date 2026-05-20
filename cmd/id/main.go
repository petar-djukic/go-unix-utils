// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd041-id R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4.
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
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `id (go-unix-utils) dev
`

type flags struct {
	user   bool
	group  bool
	groups bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	remaining, f := parseFlags(args)

	checkConflictingFlags(f)

	if len(remaining) > 0 {
		fmt.Fprintf(os.Stderr, "id: extra operand '%s'\n", remaining[0])
		fmt.Fprintln(os.Stderr, "Try 'id --help' for more information.")
		os.Exit(1)
	}

	switch {
	case f.user:
		printEffectiveUID()
	case f.group:
		printEffectiveGID()
	case f.groups:
		printGroups()
	default:
		printDefaultOutput()
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

func printEffectiveUID() {
	if _, err := fmt.Fprintln(os.Stdout, os.Getuid()); err != nil {
		os.Exit(1)
	}
}

func printEffectiveGID() {
	if _, err := fmt.Fprintln(os.Stdout, os.Getgid()); err != nil {
		os.Exit(1)
	}
}

func printGroups() {
	gids, err := syscall.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: %v\n", err)
		os.Exit(1)
	}
	parts := make([]string, len(gids))
	for i, g := range gids {
		parts[i] = strconv.Itoa(g)
	}
	if _, err := fmt.Fprintln(os.Stdout, strings.Join(parts, " ")); err != nil {
		os.Exit(1)
	}
}

func printDefaultOutput() {
	uid := os.Getuid()
	gid := os.Getgid()

	uidEntry := formatID(uid, lookupUserName(uid))
	gidEntry := formatID(gid, lookupGroupName(gid))

	gids, err := syscall.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: %v\n", err)
		os.Exit(1)
	}

	groupEntries := make([]string, 0, len(gids))
	for _, g := range gids {
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
		if arg == "--user" {
			f.user = true
			i++
			continue
		}
		if arg == "--group" {
			f.group = true
			i++
			continue
		}
		if arg == "--groups" {
			f.groups = true
			i++
			continue
		}
		if arg == "--help" {
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
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

func parseShortFlags(s string, f *flags) {
	for _, ch := range s {
		switch ch {
		case 'u':
			f.user = true
		case 'g':
			f.group = true
		case 'G':
			f.groups = true
		default:
			fmt.Fprintf(os.Stderr, "id: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'id --help' for more information.")
			os.Exit(1)
		}
	}
}
