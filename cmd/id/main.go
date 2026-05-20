// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd041-id R1.1, R1.2, R1.3, R2.1.
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
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `id (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	remaining, flagUser := parseFlags(args)

	if len(remaining) > 0 {
		fmt.Fprintf(os.Stderr, "id: extra operand '%s'\n", remaining[0])
		fmt.Fprintln(os.Stderr, "Try 'id --help' for more information.")
		os.Exit(1)
	}

	if flagUser {
		printEffectiveUID()
		return
	}

	printDefaultOutput()
}

func printEffectiveUID() {
	if _, err := fmt.Fprintln(os.Stdout, os.Getuid()); err != nil {
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

func parseFlags(args []string) ([]string, bool) {
	flagUser := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			return args[i+1:], flagUser
		}
		if arg == "--user" {
			flagUser = true
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
			parseShortFlags(arg[1:], &flagUser)
			i++
			continue
		}
		return args[i:], flagUser
	}
	return nil, flagUser
}

func parseShortFlags(flags string, flagUser *bool) {
	for _, ch := range flags {
		switch ch {
		case 'u':
			*flagUser = true
		default:
			fmt.Fprintf(os.Stderr, "id: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'id --help' for more information.")
			os.Exit(1)
		}
	}
}
