// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the id utility for printing user and group information.
//
// Implements prd041-id: default output (R1), selection flags (R2),
// modifier flags and named user support (R3).
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

	var (
		flagUser   bool
		flagGroup  bool
		flagGroups bool
		flagName   bool
		flagReal   bool
		username   string
	)

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && arg != "-" {
			for _, ch := range arg[1:] {
				switch ch {
				case 'u':
					flagUser = true
				case 'g':
					flagGroup = true
				case 'G':
					flagGroups = true
				case 'n':
					flagName = true
				case 'r':
					flagReal = true
				default:
					fmt.Fprintf(os.Stderr, "id: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
		} else {
			if username != "" {
				fmt.Fprintf(os.Stderr, "id: extra operand '%s'\n", arg)
				os.Exit(1)
			}
			username = arg
		}
	}

	// Validate flag combinations.
	selCount := 0
	if flagUser {
		selCount++
	}
	if flagGroup {
		selCount++
	}
	if flagGroups {
		selCount++
	}
	if selCount > 1 {
		fmt.Fprintf(os.Stderr, "id: cannot print \"only\" of more than one choice\n")
		os.Exit(1)
	}

	if flagName && selCount == 0 {
		fmt.Fprintf(os.Stderr, "id: cannot print only names or real IDs in default format\n")
		os.Exit(1)
	}

	if flagReal && !flagUser && !flagGroup {
		fmt.Fprintf(os.Stderr, "id: cannot print only names or real IDs in default format\n")
		os.Exit(1)
	}

	if username != "" {
		printForUser(username, flagUser, flagGroup, flagGroups, flagName)
	} else {
		printForCurrentProcess(flagUser, flagGroup, flagGroups, flagName, flagReal)
	}
}

func printForCurrentProcess(flagUser, flagGroup, flagGroups, flagName, flagReal bool) {
	uid := syscall.Geteuid()
	gid := syscall.Getegid()

	if flagReal {
		uid = syscall.Getuid()
		gid = syscall.Getgid()
	}

	if flagUser {
		printID(uid, flagName, true)
		return
	}

	if flagGroup {
		printID(gid, flagName, false)
		return
	}

	if flagGroups {
		printAllGroups(flagName)
		return
	}

	// Default: full output.
	euid := syscall.Geteuid()
	egid := syscall.Getegid()

	uidName := lookupUserName(euid)
	gidName := lookupGroupName(egid)

	fmt.Printf("uid=%d(%s) gid=%d(%s)", euid, uidName, egid, gidName)

	groups, err := syscall.Getgroups()
	if err == nil && len(groups) > 0 {
		fmt.Print(" groups=")
		for i, g := range groups {
			if i > 0 {
				fmt.Print(",")
			}
			gName := lookupGroupName(g)
			fmt.Printf("%d(%s)", g, gName)
		}
	}
	fmt.Println()
}

func printForUser(username string, flagUser, flagGroup, flagGroups, flagName bool) {
	u, err := user.Lookup(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: '%s': no such user\n", username)
		os.Exit(1)
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	if flagUser {
		printID(uid, flagName, true)
		return
	}

	if flagGroup {
		printID(gid, flagName, false)
		return
	}

	if flagGroups {
		printUserAllGroups(u, flagName)
		return
	}

	// Default output.
	uidName := u.Username
	gidName := lookupGroupName(gid)

	fmt.Printf("uid=%d(%s) gid=%d(%s)", uid, uidName, gid, gidName)

	gids, err := u.GroupIds()
	if err == nil && len(gids) > 0 {
		fmt.Print(" groups=")
		for i, gidStr := range gids {
			if i > 0 {
				fmt.Print(",")
			}
			g, _ := strconv.Atoi(gidStr)
			gName := lookupGroupName(g)
			fmt.Printf("%d(%s)", g, gName)
		}
	}
	fmt.Println()
}

func printID(id int, nameOnly, isUser bool) {
	if nameOnly {
		if isUser {
			fmt.Println(lookupUserName(id))
		} else {
			fmt.Println(lookupGroupName(id))
		}
	} else {
		fmt.Println(id)
	}
}

func printAllGroups(nameOnly bool) {
	groups, err := syscall.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: cannot get groups: %v\n", err)
		os.Exit(1)
	}

	parts := make([]string, len(groups))
	for i, g := range groups {
		if nameOnly {
			parts[i] = lookupGroupName(g)
		} else {
			parts[i] = strconv.Itoa(g)
		}
	}
	fmt.Println(strings.Join(parts, " "))
}

func printUserAllGroups(u *user.User, nameOnly bool) {
	gids, err := u.GroupIds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "id: cannot get groups: %v\n", err)
		os.Exit(1)
	}

	parts := make([]string, len(gids))
	for i, gidStr := range gids {
		g, _ := strconv.Atoi(gidStr)
		if nameOnly {
			parts[i] = lookupGroupName(g)
		} else {
			parts[i] = gidStr
		}
	}
	fmt.Println(strings.Join(parts, " "))
}

func lookupUserName(uid int) string {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return strconv.Itoa(uid)
	}
	return u.Username
}

func lookupGroupName(gid int) string {
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return strconv.Itoa(gid)
	}
	return g.Name
}
