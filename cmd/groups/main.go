// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/groups: print group memberships.
// Implements srd043-groups R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "groups"

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: groups [OPTION]... [USERNAME]...
Print group memberships for each USERNAME or, if no USERNAME is specified,
for the current process.

      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	code := run(os.Args[1:])
	if code != 0 {
		os.Exit(code)
	}
}

// run processes arguments and prints group memberships.
// R1.1: no arguments prints current user's groups.
// R1.2: with USER arguments, prints groups for each user.
// R1.3: nonexistent users produce stderr errors; exit 1 if any fail.
func run(args []string) int {
	users, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}

	if len(users) == 0 {
		return printCurrentUserGroups()
	}
	return printNamedUserGroups(users)
}

// parseArgs validates flags and returns the list of usernames.
// R2.1: --help and --version are the only accepted flags.
func parseArgs(args []string) ([]string, error) {
	var users []string
	for _, arg := range args {
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println(versionText)
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return nil, fmt.Errorf("unrecognized option '%s'", arg)
		}
		users = append(users, arg)
	}
	return users, nil
}

// printCurrentUserGroups prints groups for the current process with no prefix.
// R1.1, R2.1: uses getgroups() syscall to match GNU groups behavior.
func printCurrentUserGroups() int {
	gids, err := syscall.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get groups: %v\n", progName, err)
		return 1
	}
	names := gidSliceToNames(gids)
	fmt.Println(strings.Join(names, " "))
	return 0
}

// gidSliceToNames converts numeric GIDs to group names.
// Falls back to the numeric string if lookup fails.
func gidSliceToNames(gids []int) []string {
	names := make([]string, 0, len(gids))
	for _, gid := range gids {
		g, err := user.LookupGroupId(fmt.Sprintf("%d", gid))
		if err != nil {
			names = append(names, fmt.Sprintf("%d", gid))
			continue
		}
		names = append(names, g.Name)
	}
	return names
}

// printNamedUserGroups prints groups for each named user with "user : " prefix.
// R1.2, R1.3, R2.2: one line per user, errors to stderr, exit 1 if any fail.
func printNamedUserGroups(users []string) int {
	exitCode := 0
	for _, name := range users {
		u, err := user.Lookup(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: '%s': no such user\n", progName, name)
			exitCode = 1
			continue
		}
		groups, err := groupNamesForUser(u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: '%s': %v\n", progName, name, err)
			exitCode = 1
			continue
		}
		fmt.Printf("%s : %s\n", name, strings.Join(groups, " "))
	}
	return exitCode
}

// groupNamesForUser returns group names for the given user.
// R2.3: uses os/user package for group lookups.
func groupNamesForUser(u *user.User) ([]string, error) {
	gids, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("failed to get group IDs: %w", err)
	}
	names := make([]string, 0, len(gids))
	for _, gid := range gids {
		g, err := user.LookupGroupId(gid)
		if err != nil {
			names = append(names, gid)
			continue
		}
		names = append(names, g.Name)
	}
	return names, nil
}
