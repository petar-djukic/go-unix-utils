// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/id implements GNU id: print user and group information.
//
// Implements prd041-id R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "id"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// options holds parsed command-line flags.
type options struct {
	showUser   bool // -u / --user
	showGroup  bool // -g / --group
	showGroups bool // -G / --groups
	showName   bool // -n / --name
	help       bool
	version    bool
	userName   string // positional USER operand
}

// run parses arguments and prints identity information.
// Returns exit code: 0 on success, 1 on error.
func run(args []string, stdout, stderr *os.File) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)           //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", programName) //nolint:errcheck
		return 1
	}

	if opts.help {
		fmt.Fprint(stdout, helpText) //nolint:errcheck
		return 0
	}
	if opts.version {
		fmt.Fprint(stdout, versionText) //nolint:errcheck
		return 0
	}

	return dispatch(opts, stdout, stderr)
}

// dispatch routes to the appropriate print function based on flags.
func dispatch(opts options, stdout, stderr *os.File) int {
	switch {
	case opts.showUser:
		return printEffectiveUID(opts, stdout, stderr)
	case opts.showGroup:
		return printEffectiveGID(opts, stdout, stderr)
	case opts.showGroups:
		return printAllGroups(opts, stdout, stderr)
	default:
		return printDefaultID(opts, stdout, stderr)
	}
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (options, error) {
	var opts options
	for _, arg := range args {
		if err := parseOneArg(arg, &opts); err != nil {
			return opts, err
		}
		if opts.help || opts.version {
			return opts, nil
		}
	}
	return opts, validateOpts(opts)
}

// parseOneArg handles a single CLI argument.
func parseOneArg(arg string, opts *options) error {
	switch arg {
	case "-u", "--user":
		opts.showUser = true
	case "-g", "--group":
		opts.showGroup = true
	case "-G", "--groups":
		opts.showGroups = true
	case "-n", "--name":
		opts.showName = true
	case "--help":
		opts.help = true
	case "--version":
		opts.version = true
	default:
		return parseDefault(arg, opts)
	}
	return nil
}

// parseDefault handles non-flag arguments and unknown flags.
func parseDefault(arg string, opts *options) error {
	if len(arg) > 0 && arg[0] == '-' {
		return fmt.Errorf("invalid option -- '%s'", strings.TrimLeft(arg, "-"))
	}
	if opts.userName != "" {
		return fmt.Errorf("extra operand '%s'", arg)
	}
	opts.userName = arg
	return nil
}

// validateOpts checks for conflicting flag combinations.
// R2.4: only one of -u, -g, -G allowed.
// R3.1: -n requires a selection flag.
func validateOpts(opts options) error {
	selCount := countSelections(opts)
	if selCount > 1 {
		return fmt.Errorf("cannot print \"only\" of more than one choice")
	}
	if opts.showName && selCount == 0 {
		return fmt.Errorf("cannot print only names or real IDs in default format")
	}
	return nil
}

// countSelections returns how many selection flags are set.
func countSelections(opts options) int {
	count := 0
	if opts.showUser {
		count++
	}
	if opts.showGroup {
		count++
	}
	if opts.showGroups {
		count++
	}
	return count
}

// lookupUser resolves the user either by name or returns the current user.
func lookupUser(name string, stderr *os.File) (*user.User, error) {
	if name != "" {
		u, err := user.Lookup(name)
		if err != nil {
			fmt.Fprintf(stderr, "%s: '%s': no such user\n", programName, name) //nolint:errcheck
			return nil, err
		}
		return u, nil
	}
	return user.Current()
}

// printDefaultID prints the full identity line: uid=N(name) gid=N(name) groups=...
// R1.1: default format, R1.2: all supplementary groups, R1.3: exit 0 on success.
func printDefaultID(opts options, stdout, stderr *os.File) int {
	u, err := lookupUser(opts.userName, stderr)
	if err != nil {
		return 1
	}

	gname := lookupGroupName(u.Gid)

	groups, err := groupsForUser(u, opts.userName == "")
	if err != nil {
		fmt.Fprintf(stderr, "%s: failed to get group list: %v\n", programName, err) //nolint:errcheck
		return 1
	}

	groupParts := formatGroupList(groups)

	fmt.Fprintf(stdout, "uid=%s(%s) gid=%s(%s) groups=%s\n",
		u.Uid, u.Username, u.Gid, gname, groupParts) //nolint:errcheck

	return 0
}

// printEffectiveUID prints only the effective UID.
// R2.1: -u prints effective UID. R3.1: with -n, prints name.
func printEffectiveUID(opts options, stdout, stderr *os.File) int {
	u, err := lookupUser(opts.userName, stderr)
	if err != nil {
		return 1
	}
	if opts.showName {
		fmt.Fprintln(stdout, u.Username) //nolint:errcheck
	} else {
		fmt.Fprintln(stdout, u.Uid) //nolint:errcheck
	}
	return 0
}

// printEffectiveGID prints only the effective GID.
// R2.2: -g prints effective GID. R3.1: with -n, prints group name.
func printEffectiveGID(opts options, stdout, stderr *os.File) int {
	u, err := lookupUser(opts.userName, stderr)
	if err != nil {
		return 1
	}
	if opts.showName {
		fmt.Fprintln(stdout, lookupGroupName(u.Gid)) //nolint:errcheck
	} else {
		fmt.Fprintln(stdout, u.Gid) //nolint:errcheck
	}
	return 0
}

// printAllGroups prints all group IDs space-separated.
// R2.3: -G prints all group IDs. R3.1: with -n, prints names.
func printAllGroups(opts options, stdout, stderr *os.File) int {
	u, err := lookupUser(opts.userName, stderr)
	if err != nil {
		return 1
	}

	groups, err := groupsForUser(u, opts.userName == "")
	if err != nil {
		fmt.Fprintf(stderr, "%s: failed to get group list: %v\n", programName, err) //nolint:errcheck
		return 1
	}

	if opts.showName {
		fmt.Fprintln(stdout, formatGroupNames(groups)) //nolint:errcheck
	} else {
		fmt.Fprintln(stdout, strings.Join(groups, " ")) //nolint:errcheck
	}
	return 0
}

// lookupGroupName returns the group name for a GID, or the GID string if lookup fails.
func lookupGroupName(gid string) string {
	g, err := user.LookupGroupId(gid)
	if err != nil {
		return gid
	}
	return g.Name
}

// processGroupIDs returns the current process's supplementary group IDs
// using the getgroups() syscall, matching GNU id behavior.
func processGroupIDs() ([]string, error) {
	gids, err := os.Getgroups()
	if err != nil {
		return nil, err
	}
	result := make([]string, len(gids))
	for i, gid := range gids {
		result[i] = strconv.Itoa(gid)
	}
	return result, nil
}

// groupsForUser returns group IDs for the specified user.
// For the current process user, uses os.Getgroups() to match GNU id.
// For a named user, uses user.GroupIds() from the user database.
func groupsForUser(u *user.User, isCurrentUser bool) ([]string, error) {
	if isCurrentUser {
		return processGroupIDs()
	}
	return u.GroupIds()
}

// formatGroupList builds the comma-separated groups string: N(name),N(name),...
func formatGroupList(gids []string) string {
	parts := make([]string, 0, len(gids))
	for _, gid := range gids {
		gname := lookupGroupName(gid)
		parts = append(parts, gid+"("+gname+")")
	}
	return strings.Join(parts, ",")
}

// formatGroupNames builds a space-separated string of group names.
func formatGroupNames(gids []string) string {
	names := make([]string, len(gids))
	for i, gid := range gids {
		names[i] = lookupGroupName(gid)
	}
	return strings.Join(names, " ")
}

const helpText = `Usage: id [OPTION]... [USER]
Print user and group information for each specified USER,
or (when USER omitted) for the current process.

  -u, --user     print only the effective user ID
  -g, --group    print only the effective group ID
  -G, --groups   print all group IDs
  -n, --name     print a name instead of a number, for -ugG
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = "id (go-unix-utils) 0.1\n"
