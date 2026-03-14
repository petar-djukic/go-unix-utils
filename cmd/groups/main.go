// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd043-groups R1.1, R1.2, R1.3, R2.1, R2.2, R2.3
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and version messages.
const programName = "groups"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: check for --help and --version before processing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Print(helpText) //nolint:errcheck
			return
		case "--version":
			fmt.Println("groups (go-unix-utils) 0.1") //nolint:errcheck
			return
		}
	}

	// R1.1, R2.1: no arguments — print current process's groups via getgroups().
	// GNU groups with no args uses getgroups() syscall, not the user database.
	if len(args) == 0 {
		gids, err := os.Getgroups()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to get group list\n", programName)
			os.Exit(1)
		}
		gidStrings := make([]string, len(gids))
		for i, gid := range gids {
			gidStrings[i] = fmt.Sprintf("%d", gid)
		}
		names := groupIDsToNames(gidStrings)
		fmt.Println(strings.Join(names, " "))
		return
	}

	// R1.2, R2.2, R1.3: one or more USER arguments.
	exitCode := 0
	for _, username := range args {
		u, err := user.Lookup(username)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: '%s': no such user\n", programName, username)
			exitCode = 1
			continue
		}
		groups, err := u.GroupIds()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: failed to get groups for '%s'\n", programName, username)
			exitCode = 1
			continue
		}
		names := groupIDsToNames(groups)
		// R2.2: "user : group1 group2 ..."
		fmt.Printf("%s : %s\n", username, strings.Join(names, " "))
	}
	os.Exit(exitCode)
}

// groupIDsToNames converts a slice of group ID strings to group names.
// If a GID cannot be resolved, the numeric GID is used as-is.
func groupIDsToNames(gids []string) []string {
	names := make([]string, 0, len(gids))
	for _, gid := range gids {
		g, err := user.LookupGroupId(gid)
		if err != nil {
			names = append(names, gid)
			continue
		}
		names = append(names, g.Name)
	}
	return names
}

// helpText is the usage message printed by --help.
const helpText = `Usage: groups [OPTION]... [USERNAME]...
Print group memberships for each USERNAME or, if no USERNAME is specified, for
the current process (which may differ if the groups database has changed).

      --help     display this help and exit
      --version  output version information and exit
`
