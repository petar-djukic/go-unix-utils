// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd043-groups R1.1 (current user groups), R1.2 (named user groups),
// R1.3 (nonexistent user error handling), R2.1 (no-prefix current user output),
// R2.2 ("user : groups" prefix format for named users),
// R2.3 (group names in system database order).
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "groups"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints group memberships.
// Returns the exit code.
func run(args []string) int {
	users := parseArgs(args)
	if len(users) == 0 {
		return printCurrentUserGroups()
	}
	return printNamedUserGroups(users)
}

// parseArgs extracts user names from arguments, handling --help/--version.
func parseArgs(args []string) []string {
	var users []string
	for _, arg := range args {
		if arg == "--help" {
			fmt.Print(helpText())
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("groups (go-unix-utils) 1.0")
			os.Exit(0)
		}
		users = append(users, arg)
	}
	return users
}

// printCurrentUserGroups prints groups for the current user with no prefix.
// R1.1, R2.1: space-separated group names, single line, exit 0.
// Uses os.Getgroups() (getgroups(2)) to match GNU groups process group list.
func printCurrentUserGroups() int {
	gids, err := os.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to get groups: %v\n",
			programName, err)
		return 1
	}
	names := make([]string, len(gids))
	for i, gid := range gids {
		names[i] = lookupGroupName(strconv.Itoa(gid))
	}
	fmt.Println(strings.Join(names, " "))
	return 0
}

// printNamedUserGroups prints groups for each named user with "user : " prefix.
// R1.2, R1.3, R2.2: one line per user; exit 1 if any user not found.
func printNamedUserGroups(users []string) int {
	exitCode := 0
	for _, username := range users {
		u, err := user.Lookup(username)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: '%s': no such user\n",
				programName, username)
			exitCode = 1
			continue
		}
		names, err := lookupUserGroupNames(u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to get groups for '%s': %v\n",
				programName, username, err)
			exitCode = 1
			continue
		}
		fmt.Printf("%s : %s\n", username, strings.Join(names, " "))
	}
	return exitCode
}

// lookupUserGroupNames returns group names for the given user.
// R2.3: group names appear in system order.
func lookupUserGroupNames(u *user.User) ([]string, error) {
	gids, err := u.GroupIds()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(gids))
	for i, gid := range gids {
		names[i] = lookupGroupName(gid)
	}
	return names, nil
}

// lookupGroupName returns the group name for a GID string.
// Returns the GID string itself if lookup fails.
func lookupGroupName(gid string) string {
	g, err := user.LookupGroupId(gid)
	if err != nil {
		return gid
	}
	return g.Name
}

// helpText returns the --help output.
func helpText() string {
	return `Usage: groups [OPTION]... [USERNAME]...
Print group memberships for each USERNAME or, if no USERNAME is specified,
for the current process.

      --help     display this help and exit
      --version  output version information and exit
`
}
