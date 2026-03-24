// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd043-groups: Print Group Memberships.
// Covers R1.1-R1.3 (default behavior, named users, error handling),
// R2.1-R2.3 (output format, prefix, group ordering).
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "groups"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints group memberships. Returns exit code.
// R1.1: no args prints current user's groups.
// R1.2: with USER args, prints each user's groups with prefix.
// R1.3: exits 1 if any user lookup fails.
func run(args []string) int {
	for _, arg := range args {
		switch arg {
		case "--help":
			return printHelp()
		case "--version":
			return printVersion()
		}
	}

	if len(args) == 0 {
		return printCurrentUserGroups()
	}
	return printNamedUserGroups(args)
}

// printCurrentUserGroups prints the current user's groups with no prefix.
// R2.1: space-separated group names, no prefix.
func printCurrentUserGroups() int {
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	names, err := groupNamesForUser(u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	if _, err := fmt.Println(strings.Join(names, " ")); err != nil {
		return 1
	}
	return 0
}

// printNamedUserGroups prints groups for each named user with prefix.
// R2.2: format "user : group1 group2 ...".
// R2.3: continues on error, returns 1 if any lookup failed.
func printNamedUserGroups(usernames []string) int {
	exitCode := 0
	for _, username := range usernames {
		if err := printOneNamedUser(username); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: no such user\n",
				progName, username)
			exitCode = 1
		}
	}
	return exitCode
}

// printOneNamedUser looks up and prints groups for a single named user.
func printOneNamedUser(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	names, err := groupNamesForUser(u)
	if err != nil {
		return err
	}
	if _, err := fmt.Printf("%s : %s\n", username, strings.Join(names, " ")); err != nil {
		return err
	}
	return nil
}

// groupNamesForUser returns group names for a user in system order.
// R2.3: primary group first, then supplementary groups.
func groupNamesForUser(u *user.User) ([]string, error) {
	gidStrs, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("getting groups: %w", err)
	}
	primaryGID := u.Gid
	return gidsToNames(primaryGID, gidStrs)
}

// gidsToNames converts GID strings to group names with primary GID first.
func gidsToNames(primaryGID string, gidStrs []string) ([]string, error) {
	ordered := prependUnique(primaryGID, gidStrs)
	names := make([]string, 0, len(ordered))
	for _, gidStr := range ordered {
		name, err := resolveGroupName(gidStr)
		if err != nil {
			// Fall back to numeric GID if name resolution fails.
			names = append(names, gidStr)
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// resolveGroupName looks up a group name by its numeric GID string.
func resolveGroupName(gidStr string) (string, error) {
	gid, err := strconv.Atoi(gidStr)
	if err != nil {
		return "", err
	}
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return "", err
	}
	return g.Name, nil
}

// prependUnique ensures id appears first in the slice, removing duplicates.
func prependUnique(id string, ids []string) []string {
	result := []string{id}
	for _, s := range ids {
		if s != id {
			result = append(result, s)
		}
	}
	return result
}

// printHelp writes usage information to stdout. Returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: groups [OPTION]... [USERNAME]...
Print group memberships for each USERNAME or, if no USERNAME is specified,
for the current process.

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout. Returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 1
	}
	return 0
}
