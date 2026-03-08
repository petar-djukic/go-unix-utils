// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the groups utility for printing group memberships.
//
// Implements prd043-groups: default behavior (R1), output format (R2).
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

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) == 0 {
		printCurrentUserGroups()
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

func printCurrentUserGroups() {
	gids, err := syscall.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "groups: cannot get group list: %v\n", err)
		os.Exit(1)
	}

	names := make([]string, 0, len(gids))
	for _, gid := range gids {
		g, err := user.LookupGroupId(strconv.Itoa(gid))
		if err != nil {
			names = append(names, strconv.Itoa(gid))
			continue
		}
		names = append(names, g.Name)
	}
	fmt.Println(strings.Join(names, " "))
}

func printUserGroups(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}

	gids, err := u.GroupIds()
	if err != nil {
		return err
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
	fmt.Printf("%s : %s\n", username, strings.Join(names, " "))
	return nil
}
