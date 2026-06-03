// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd043-groups R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
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

const helpText = `Usage: groups [OPTION]... [USERNAME]...
Print group memberships for each USERNAME or, if no USERNAME is specified, for
the current process (which may differ if the groups database has changed).
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `groups (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	args = parseFlags(args)

	if len(args) == 0 {
		if err := printCurrentUserGroups(); err != nil {
			fmt.Fprintf(os.Stderr, "groups: cannot find name for group ID: %v\n", err)
			os.Exit(1)
		}
		return
	}

	exitCode := 0
	for _, username := range args {
		if err := printUserGroups(username); err != nil {
			fmt.Fprintf(os.Stderr, "groups: '%s': no such user\n", username)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func printCurrentUserGroups() error {
	gids, err := syscall.Getgroups()
	if err != nil {
		return err
	}
	names, err := resolveGroupIDs(gids)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, strings.Join(names, " ")); err != nil {
		os.Exit(1)
	}
	return nil
}

func printUserGroups(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	gidStrs, err := u.GroupIds()
	if err != nil {
		return err
	}
	gids := make([]int, 0, len(gidStrs))
	for _, s := range gidStrs {
		gid, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		gids = append(gids, gid)
	}
	names, err := resolveGroupIDs(gids)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "%s : %s\n", username, strings.Join(names, " ")); err != nil {
		os.Exit(1)
	}
	return nil
}

func resolveGroupIDs(gids []int) ([]string, error) {
	names := make([]string, 0, len(gids))
	for _, gid := range gids {
		g, err := user.LookupGroupId(strconv.Itoa(gid))
		if err != nil {
			names = append(names, strconv.Itoa(gid))
			continue
		}
		names = append(names, g.Name)
	}
	return names, nil
}

func parseFlags(args []string) []string {
	for len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case "--":
			return args[1:]
		default:
			if strings.HasPrefix(args[0], "-") && len(args[0]) > 1 {
				fmt.Fprintf(os.Stderr, "groups: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'groups --help' for more information.")
				os.Exit(1)
			}
			return args
		}
	}
	return args
}
