// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/groups implements GNU groups: print group memberships.
//
// Implements prd043-groups R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "groups"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints group memberships.
// R1.1: no arguments prints current user's groups.
// R1.2: USER arguments print groups for each named user.
// R1.3: nonexistent users produce stderr errors; exit 1 if any failed.
func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		return printCurrentUserGroups(stdout, stderr)
	}
	return printNamedUserGroups(args, stdout, stderr)
}

// printCurrentUserGroups prints group names for the current user.
// R1.1: space-separated group names, no prefix, single line, exit 0.
// R2.1: no prefix when no USER argument is given.
// R2.3: group names in system group database order via getgroups(2).
func printCurrentUserGroups(stdout, stderr *os.File) int {
	gids, err := syscall.Getgroups()
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot find groups: %v\n", programName, err) //nolint:errcheck
		return 1
	}

	names := gidSliceToNames(gids)
	fmt.Fprintln(stdout, strings.Join(names, " ")) //nolint:errcheck // best-effort
	return 0
}

// gidSliceToNames converts a slice of integer GIDs to group names.
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

// printNamedUserGroups prints group memberships for each named user.
// R1.2: one line per user with "user : group1 group2" format.
// R1.3: nonexistent users produce stderr errors; exit 1 if any failed.
// R2.2: each line formatted as "user : group1 group2 ...".
// R2.3: group names in system group database order via GroupIds().
func printNamedUserGroups(users []string, stdout, stderr *os.File) int {
	exitCode := 0
	for _, username := range users {
		u, err := user.Lookup(username)
		if err != nil {
			fmt.Fprintf(stderr, "%s: '%s': no such user\n", programName, username) //nolint:errcheck
			exitCode = 1
			continue
		}

		names, err := groupNamesForUser(u)
		if err != nil {
			fmt.Fprintf(stderr, "%s: '%s': %v\n", programName, username, err) //nolint:errcheck
			exitCode = 1
			continue
		}

		fmt.Fprintf(stdout, "%s : %s\n", username, strings.Join(names, " ")) //nolint:errcheck // best-effort
	}
	return exitCode
}

// groupNamesForUser returns group names for the given user.
func groupNamesForUser(u *user.User) ([]string, error) {
	gids, err := u.GroupIds()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(gids))
	for _, gid := range gids {
		g, err := user.LookupGroupId(gid)
		if err != nil {
			// Fall back to numeric GID if name lookup fails.
			names = append(names, gid)
			continue
		}
		names = append(names, g.Name)
	}
	return names, nil
}
