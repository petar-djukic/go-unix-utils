// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd041-id R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3
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

	opts, username := parseArgs(os.Args[1:])

	// R2.4: only one of -u, -g, -G may be specified at a time.
	selCount := boolCount(opts.flagUser, opts.flagGroup, opts.flagGroups)
	if selCount > 1 {
		fmt.Fprintf(os.Stderr, "%s: cannot print \"only\" of more than one choice\n", programName)
		os.Exit(1)
	}

	// R3.1: -n without a selection flag is an error.
	if opts.flagName && selCount == 0 {
		fmt.Fprintf(os.Stderr, "%s: printing only names or real IDs requires -u, -g, or -G\n", programName)
		os.Exit(1)
	}

	// R3.2: -r without -u or -g is an error.
	if opts.flagReal && !opts.flagUser && !opts.flagGroup {
		fmt.Fprintf(os.Stderr, "%s: printing only names or real IDs requires -u, -g, or -G\n", programName)
		os.Exit(1)
	}

	if username == "" {
		if err := printCurrentUser(opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			os.Exit(1)
		}
	} else {
		if err := printNamedUser(username, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			os.Exit(1)
		}
	}
}

// options holds the parsed command-line flags.
type options struct {
	flagUser   bool // -u / --user
	flagGroup  bool // -g / --group
	flagGroups bool // -G / --groups
	flagName   bool // -n / --name
	flagReal   bool // -r / --real
}

// parseArgs parses command-line arguments and returns options and an optional username.
// Handles --help and --version directly (exits the process).
func parseArgs(args []string) (options, string) {
	var opts options
	var username string

	for _, arg := range args {
		switch {
		case arg == "--help":
			fmt.Print(helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Println("id (go-unix-utils) 0.1")
			os.Exit(0)
		case arg == "--user":
			opts.flagUser = true
		case arg == "--group":
			opts.flagGroup = true
		case arg == "--groups":
			opts.flagGroups = true
		case arg == "--name":
			opts.flagName = true
		case arg == "--real":
			opts.flagReal = true
		case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
			// R2.1-R2.3: short flags, can be combined (e.g., -un, -Gn).
			for _, ch := range arg[1:] {
				switch ch {
				case 'u':
					opts.flagUser = true
				case 'g':
					opts.flagGroup = true
				case 'G':
					opts.flagGroups = true
				case 'n':
					opts.flagName = true
				case 'r':
					opts.flagReal = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, ch)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			username = arg
		}
	}

	return opts, username
}

// boolCount returns the number of true values among its arguments.
func boolCount(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

// printCurrentUser prints identity information for the current process user
// using syscall-level uid/gid and os.Getgroups().
//
// R1.1: uid=N(name) gid=N(name) groups=N(name),N(name),...
// R2.1-R2.3: flag-specific output modes.
func printCurrentUser(opts options) error {
	uid := os.Getuid()
	gid := os.Getgid()

	uidStr := strconv.Itoa(uid)
	gidStr := strconv.Itoa(gid)

	// R2.1: -u prints only the UID.
	// R3.2: effective by default; real with -r.
	if opts.flagUser {
		id := uidStr
		if !opts.flagReal {
			id = strconv.Itoa(os.Geteuid())
		}
		if opts.flagName {
			fmt.Println(lookupUserName(id))
		} else {
			fmt.Println(id)
		}
		return nil
	}

	// R2.2: -g prints only the GID.
	// R3.2: effective by default; real with -r.
	if opts.flagGroup {
		id := gidStr
		if !opts.flagReal {
			id = strconv.Itoa(os.Getegid())
		}
		if opts.flagName {
			fmt.Println(lookupGroupName(id))
		} else {
			fmt.Println(id)
		}
		return nil
	}

	// R2.3: -G prints all group IDs, space-separated.
	if opts.flagGroups {
		gids, err := os.Getgroups()
		if err != nil {
			return fmt.Errorf("failed to get supplementary groups: %w", err)
		}
		entries := make([]string, len(gids))
		for i, g := range gids {
			gs := strconv.Itoa(g)
			if opts.flagName {
				entries[i] = lookupGroupName(gs)
			} else {
				entries[i] = gs
			}
		}
		fmt.Println(strings.Join(entries, " "))
		return nil
	}

	// R1.1: default output — full identity line.
	userName := lookupUserName(uidStr)
	groupName := lookupGroupName(gidStr)

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
// R2.1-R2.3: flag-specific output modes.
func printNamedUser(username string, opts options) error {
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("'%s': no such user", username)
	}

	// R2.1: -u prints only the effective UID.
	if opts.flagUser {
		if opts.flagName {
			fmt.Println(u.Username)
		} else {
			fmt.Println(u.Uid)
		}
		return nil
	}

	// R2.2: -g prints only the effective GID.
	if opts.flagGroup {
		if opts.flagName {
			fmt.Println(lookupGroupName(u.Gid))
		} else {
			fmt.Println(u.Gid)
		}
		return nil
	}

	// R2.3: -G prints all group IDs, space-separated.
	if opts.flagGroups {
		gids, err := u.GroupIds()
		if err != nil {
			return fmt.Errorf("failed to get groups for '%s': %w", username, err)
		}
		entries := make([]string, len(gids))
		for i, gs := range gids {
			if opts.flagName {
				entries[i] = lookupGroupName(gs)
			} else {
				entries[i] = gs
			}
		}
		fmt.Println(strings.Join(entries, " "))
		return nil
	}

	// R1.1: default output — full identity line.
	userName := u.Username
	groupName := lookupGroupName(u.Gid)

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

  -u, --user     print only the effective user ID
  -g, --group    print only the effective group ID
  -G, --groups   print all group IDs
  -n, --name     print a name instead of a number, for -ugG
  -r, --real     print the real ID instead of the effective ID, with -ug
      --help     display this help and exit
      --version  output version information and exit
`
