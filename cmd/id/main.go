// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd041-id R1.1, R1.2, R1.3
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "id"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// Handle --help and --version before processing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Print(helpText)
			return
		case "--version":
			fmt.Println("id (go-unix-utils) 0.1")
			return
		}
	}

	// R1.1, R1.2: no arguments — current user; one argument — named user.
	if len(args) == 0 {
		if err := printCurrentUser(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			os.Exit(1)
		}
		return
	}

	// R1.2: username argument provided.
	username := args[0]
	if err := printNamedUser(username); err != nil {
		// R1.3: exit 1 with error on stderr for nonexistent user.
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}
}

// printCurrentUser prints identity information for the current process user
// using syscall-level uid/gid and os.Getgroups().
//
// R1.1: uid=N(name) gid=N(name) groups=N(name),N(name),...
func printCurrentUser() error {
	uid := os.Getuid()
	gid := os.Getgid()

	uidStr := strconv.Itoa(uid)
	gidStr := strconv.Itoa(gid)

	userName := lookupUserName(uidStr)
	groupName := lookupGroupName(gidStr)

	// R1.2: supplementary groups from the process.
	gids, err := os.Getgroups()
	if err != nil {
		return fmt.Errorf("failed to get supplementary groups: %w", err)
	}

	groupEntries := make([]string, len(gids))
	for i, g := range gids {
		gs := strconv.Itoa(g)
		gn := lookupGroupName(gs)
		groupEntries[i] = formatIDEntry(gs, gn)
	}

	fmt.Printf("uid=%s gid=%s groups=%s\n",
		formatIDEntry(uidStr, userName),
		formatIDEntry(gidStr, groupName),
		strings.Join(groupEntries, ","))
	return nil
}

// printNamedUser prints identity information for the specified username.
//
// R1.2: query named user from the system user database.
// R1.3: returns error if the user does not exist.
func printNamedUser(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("'%s': no such user", username)
	}

	userName := u.Username
	groupName := lookupGroupName(u.Gid)

	// R1.2: supplementary groups from user database.
	gids, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("failed to get groups for '%s': %w", username, err)
	}

	groupEntries := make([]string, len(gids))
	for i, gs := range gids {
		gn := lookupGroupName(gs)
		groupEntries[i] = formatIDEntry(gs, gn)
	}

	fmt.Printf("uid=%s gid=%s groups=%s\n",
		formatIDEntry(u.Uid, userName),
		formatIDEntry(u.Gid, groupName),
		strings.Join(groupEntries, ","))
	return nil
}

// formatIDEntry formats an id entry as "N(name)".
func formatIDEntry(id string, name string) string {
	return fmt.Sprintf("%s(%s)", id, name)
}

// lookupUserName resolves a numeric UID string to a username.
// Returns the numeric string if lookup fails.
func lookupUserName(uid string) string {
	u, err := user.LookupId(uid)
	if err != nil {
		return uid
	}
	return u.Username
}

// lookupGroupName resolves a numeric GID string to a group name.
// Returns the numeric string if lookup fails.
func lookupGroupName(gid string) string {
	g, err := user.LookupGroupId(gid)
	if err != nil {
		return gid
	}
	return g.Name
}

// helpText is the usage message printed by --help.
const helpText = `Usage: id [OPTION]... [USER]
Print user and group information for each specified USER,
or (when USER omitted) for the current process.

      --help     display this help and exit
      --version  output version information and exit
`
