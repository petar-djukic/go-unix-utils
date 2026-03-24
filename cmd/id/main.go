// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd041-id: Print User and Group Information.
// Covers R1.1-R1.3 (default output, group ordering, exit code),
// R2.1-R2.3 (-u, -g, -G selection flags),
// R2.4 (conflicting flag detection),
// R3.1-R3.2 (-n name modifier, -r real ID modifier).
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "id"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// options holds parsed command-line flags.
type options struct {
	showUser   bool // -u / --user
	showGroup  bool // -g / --group
	showGroups bool // -G / --groups
	showName   bool // -n / --name
	showReal   bool // -r / --real
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints identity information. Returns exit code.
// R1.3: returns 0 on success.
func run(args []string) int {
	for _, a := range args {
		switch a {
		case "--help":
			return printHelp()
		case "--version":
			return printVersion()
		}
	}
	opts, err := parseArgs(args)
	if err != nil {
		printArgError(err)
		return 1
	}
	if err := validateOpts(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	return printIdentity(opts)
}

// printArgError prints a flag-parsing error with a usage hint.
func printArgError(err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (options, error) {
	var opts options
	for _, arg := range args {
		if err := parseOneArg(arg, &opts); err != nil {
			return opts, err
		}
	}
	return opts, nil
}

// parseOneArg processes a single command-line argument.
func parseOneArg(arg string, opts *options) error {
	switch {
	case arg == "--user":
		opts.showUser = true
	case arg == "--group":
		opts.showGroup = true
	case arg == "--groups":
		opts.showGroups = true
	case arg == "--name":
		opts.showName = true
	case arg == "--real":
		opts.showReal = true
	case arg == "--help", arg == "--version":
		// Already handled in run().
	case strings.HasPrefix(arg, "--"):
		return fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(arg[1:], opts)
	}
	return nil
}

// parseShortFlags parses combined short flags like "-un" or "-Gn".
func parseShortFlags(flags string, opts *options) error {
	for _, c := range flags {
		switch c {
		case 'u':
			opts.showUser = true
		case 'g':
			opts.showGroup = true
		case 'G':
			opts.showGroups = true
		case 'n':
			opts.showName = true
		case 'r':
			opts.showReal = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// validateOpts checks for conflicting flag combinations.
// R2.4: only one selection flag allowed.
// R3.1: -n requires a selection flag.
// R3.2: -r requires -u or -g.
func validateOpts(opts options) error {
	selCount := countTrue(opts.showUser, opts.showGroup, opts.showGroups)
	if selCount > 1 {
		return fmt.Errorf("cannot print \"only\" of more than one choice")
	}
	if opts.showName && selCount == 0 {
		return fmt.Errorf(
			"cannot print only names or real IDs in default format")
	}
	if opts.showReal && !opts.showUser && !opts.showGroup {
		return fmt.Errorf(
			"cannot print only names or real IDs in default format")
	}
	return nil
}

// countTrue returns the number of true values.
func countTrue(vals ...bool) int {
	n := 0
	for _, v := range vals {
		if v {
			n++
		}
	}
	return n
}

// printIdentity dispatches to the appropriate formatter and prints output.
func printIdentity(opts options) int {
	var output string
	var err error
	switch {
	case opts.showUser:
		output, err = formatUser(opts)
	case opts.showGroup:
		output, err = formatGroup(opts)
	case opts.showGroups:
		output, err = formatGroups(opts)
	default:
		output, err = formatDefault()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	if _, err := fmt.Println(output); err != nil {
		return 1
	}
	return 0
}

// formatDefault produces the full identity line.
// R1.1: uid=N(name) gid=N(name) groups=N(name),...
func formatDefault() (string, error) {
	uid := os.Geteuid()
	gid := os.Getegid()
	uname, err := lookupUsername(uid)
	if err != nil {
		return "", err
	}
	gname, err := lookupGroupName(gid)
	if err != nil {
		return "", err
	}
	groupStr, err := formatGroupEntries()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("uid=%d(%s) gid=%d(%s) groups=%s",
		uid, uname, gid, gname, groupStr), nil
}

// formatGroupEntries builds the comma-separated groups list for default output.
// R1.2: includes all supplementary groups in system order.
func formatGroupEntries() (string, error) {
	gids, err := getGroupIDs()
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(gids))
	for _, g := range gids {
		name, lookupErr := lookupGroupName(g)
		if lookupErr != nil {
			parts = append(parts, strconv.Itoa(g))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d(%s)", g, name))
	}
	return strings.Join(parts, ","), nil
}

// formatUser produces output for -u flag.
// R2.1: prints effective UID (or real with -r, name with -n).
func formatUser(opts options) (string, error) {
	uid := os.Geteuid()
	if opts.showReal {
		uid = os.Getuid()
	}
	if opts.showName {
		return lookupUsername(uid)
	}
	return strconv.Itoa(uid), nil
}

// formatGroup produces output for -g flag.
// R2.2: prints effective GID (or real with -r, name with -n).
func formatGroup(opts options) (string, error) {
	gid := os.Getegid()
	if opts.showReal {
		gid = os.Getgid()
	}
	if opts.showName {
		return lookupGroupName(gid)
	}
	return strconv.Itoa(gid), nil
}

// formatGroups produces output for -G flag.
// R2.3: prints all group IDs space-separated (names with -n).
func formatGroups(opts options) (string, error) {
	gids, err := getGroupIDs()
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(gids))
	for _, g := range gids {
		s, fmtErr := formatOneGroupID(g, opts.showName)
		if fmtErr != nil {
			parts = append(parts, strconv.Itoa(g))
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " "), nil
}

// formatOneGroupID formats a single group ID as name or number.
func formatOneGroupID(gid int, showName bool) (string, error) {
	if showName {
		return lookupGroupName(gid)
	}
	return strconv.Itoa(gid), nil
}

// getGroupIDs returns all group IDs with the effective GID first.
// R1.2: effective GID appears first, followed by supplementary groups.
func getGroupIDs() ([]int, error) {
	egid := os.Getegid()
	gids, err := os.Getgroups()
	if err != nil {
		return nil, fmt.Errorf("getting groups: %w", err)
	}
	return prependUnique(egid, gids), nil
}

// prependUnique ensures id appears first, removing duplicates.
func prependUnique(id int, ids []int) []int {
	result := []int{id}
	for _, g := range ids {
		if g != id {
			result = append(result, g)
		}
	}
	return result
}

// lookupUsername resolves a numeric UID to a username.
func lookupUsername(uid int) (string, error) {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", fmt.Errorf("cannot find name for user ID %d", uid)
	}
	return u.Username, nil
}

// lookupGroupName resolves a numeric GID to a group name.
func lookupGroupName(gid int) (string, error) {
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return "", fmt.Errorf("cannot find name for group ID %d", gid)
	}
	return g.Name, nil
}

// printHelp writes usage information to stdout. Returns exit code.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: id [OPTION]... [USER]
Print user and group information for USER, or for the current process.

  -u, --user     print only the effective user ID
  -g, --group    print only the effective group ID
  -G, --groups   print all group IDs
  -n, --name     print a name instead of a number, for -ugG
  -r, --real     print the real ID instead of the effective ID, with -ug
      --help     display this help and exit
      --version  output version information and exit
`)
	return 0
}

// printVersion writes version information to stdout. Returns exit code.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	return 0
}
