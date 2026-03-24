// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd041-id: Print User and Group Information.
// Covers R1.1-R1.3 (default output, group ordering, exit code),
// R2.1-R2.4 (-u, -g, -G selection flags, conflicting flag detection),
// R3.1-R3.3 (-n name modifier, -r real ID modifier, named user support).
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
	showUser   bool   // -u / --user
	showGroup  bool   // -g / --group
	showGroups bool   // -G / --groups
	showName   bool   // -n / --name
	showReal   bool   // -r / --real
	username   string // R3.3: optional USER operand
}

// userInfo holds resolved identity information for output formatting.
type userInfo struct {
	uid     int   // effective UID (or named user's UID)
	gid     int   // effective GID (or named user's GID)
	realUID int   // real UID (same as uid for named users)
	realGID int   // real GID (same as gid for named users)
	groups  []int // all group IDs with primary first
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints identity information. Returns exit code.
// R1.3, R3.1: returns 0 on success, 1 on failure.
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
// R3.3: non-flag arguments are treated as a username operand.
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
	default:
		// R3.3: positional argument is a username.
		if opts.username != "" {
			return fmt.Errorf("extra operand '%s'", arg)
		}
		opts.username = arg
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

// resolveUser resolves identity information for the target user.
// R3.3: when username is set, looks up from the system user database.
func resolveUser(opts options) (*userInfo, error) {
	if opts.username != "" {
		return resolveNamedUser(opts.username)
	}
	return resolveCurrentUser()
}

// resolveNamedUser looks up a user by name from the system database.
// R3.3: returns error if user does not exist.
func resolveNamedUser(username string) (*userInfo, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("'%s': no such user", username)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	groups, err := resolveNamedUserGroups(u, gid)
	if err != nil {
		return nil, err
	}
	return &userInfo{
		uid: uid, gid: gid,
		realUID: uid, realGID: gid,
		groups: groups,
	}, nil
}

// resolveNamedUserGroups returns group IDs for a named user with GID first.
func resolveNamedUserGroups(u *user.User, primaryGID int) ([]int, error) {
	gidStrs, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("getting groups: %w", err)
	}
	gids := make([]int, 0, len(gidStrs))
	for _, s := range gidStrs {
		g, convErr := strconv.Atoi(s)
		if convErr != nil {
			continue
		}
		gids = append(gids, g)
	}
	return prependUnique(primaryGID, gids), nil
}

// resolveCurrentUser resolves identity from the current process.
func resolveCurrentUser() (*userInfo, error) {
	groups, err := getCurrentGroups()
	if err != nil {
		return nil, err
	}
	return &userInfo{
		uid: os.Geteuid(), gid: os.Getegid(),
		realUID: os.Getuid(), realGID: os.Getgid(),
		groups: groups,
	}, nil
}

// getCurrentGroups returns process group IDs with effective GID first.
// R1.2: effective GID appears first, followed by supplementary groups.
func getCurrentGroups() ([]int, error) {
	egid := os.Getegid()
	gids, err := os.Getgroups()
	if err != nil {
		return nil, fmt.Errorf("getting groups: %w", err)
	}
	return prependUnique(egid, gids), nil
}

// printIdentity dispatches to the appropriate formatter and prints output.
func printIdentity(opts options) int {
	info, err := resolveUser(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	output, err := formatOutput(info, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	if _, err := fmt.Println(output); err != nil {
		return 1
	}
	return 0
}

// formatOutput selects the appropriate output format based on flags.
func formatOutput(info *userInfo, opts options) (string, error) {
	switch {
	case opts.showUser:
		return formatUser(info, opts), nil
	case opts.showGroup:
		return formatGroup(info, opts), nil
	case opts.showGroups:
		return formatGroups(info, opts), nil
	default:
		return formatDefault(info)
	}
}

// formatDefault produces the full identity line.
// R1.1: uid=N(name) gid=N(name) groups=N(name),...
func formatDefault(info *userInfo) (string, error) {
	uname, err := lookupUsername(info.uid)
	if err != nil {
		return "", err
	}
	gname, err := lookupGroupName(info.gid)
	if err != nil {
		return "", err
	}
	groupStr := formatGroupEntries(info.groups)
	return fmt.Sprintf("uid=%d(%s) gid=%d(%s) groups=%s",
		info.uid, uname, info.gid, gname, groupStr), nil
}

// formatGroupEntries builds the comma-separated groups list for default output.
// R1.2: includes all supplementary groups in system order.
func formatGroupEntries(gids []int) string {
	parts := make([]string, 0, len(gids))
	for _, g := range gids {
		name, err := lookupGroupName(g)
		if err != nil {
			parts = append(parts, strconv.Itoa(g))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d(%s)", g, name))
	}
	return strings.Join(parts, ",")
}

// formatUser produces output for -u flag.
// R2.1: prints UID (real with -r, name with -n).
func formatUser(info *userInfo, opts options) string {
	uid := info.uid
	if opts.showReal {
		uid = info.realUID
	}
	if opts.showName {
		name, err := lookupUsername(uid)
		if err != nil {
			return strconv.Itoa(uid)
		}
		return name
	}
	return strconv.Itoa(uid)
}

// formatGroup produces output for -g flag.
// R2.2: prints GID (real with -r, name with -n).
func formatGroup(info *userInfo, opts options) string {
	gid := info.gid
	if opts.showReal {
		gid = info.realGID
	}
	if opts.showName {
		name, err := lookupGroupName(gid)
		if err != nil {
			return strconv.Itoa(gid)
		}
		return name
	}
	return strconv.Itoa(gid)
}

// formatGroups produces output for -G flag.
// R2.3: prints all group IDs space-separated (names with -n).
func formatGroups(info *userInfo, opts options) string {
	parts := make([]string, 0, len(info.groups))
	for _, g := range info.groups {
		parts = append(parts, formatOneGroupID(g, opts.showName))
	}
	return strings.Join(parts, " ")
}

// formatOneGroupID formats a single group ID as name or number.
func formatOneGroupID(gid int, showName bool) string {
	if showName {
		name, err := lookupGroupName(gid)
		if err != nil {
			return strconv.Itoa(gid)
		}
		return name
	}
	return strconv.Itoa(gid)
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
