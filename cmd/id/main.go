// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/id: print user and group information.
// Implements srd041-id R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
// NOTE: -Z/--context (SELinux) is a non_goal per srd041 and is not implemented.
package main

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "id"

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: id [OPTION]... [USER]
Print user and group information for each specified USER,
or (when USER omitted) for the current process.

  -u, --user     print only the effective user ID
  -g, --group    print only the effective group ID
  -G, --groups   print all group IDs
  -n, --name     print a name instead of a number, for -ugG
  -r, --real     print the real ID instead of the effective ID, for -ug
  -z, --zero     delimit entries with NUL characters, not whitespace
      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	code := run(os.Args[1:])
	if code != 0 {
		os.Exit(code)
	}
}

// opts holds the parsed command-line flags.
type opts struct {
	flagU    bool
	flagG    bool
	flagBigG bool
	flagN    bool
	flagR    bool
	flagZero bool
	user     string
}

// run processes arguments and prints identity information.
func run(args []string) int {
	o, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}

	if err := validateFlags(o); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}

	if err := printID(o); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	return 0
}

// parseArgs parses command-line arguments into opts.
func parseArgs(args []string) (*opts, error) {
	o := &opts{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println(versionText)
			os.Exit(0)
		}
		if err := parseLongFlag(o, arg); err == nil {
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" && !strings.HasPrefix(arg, "--") {
			if err := parseShortFlags(o, arg[1:]); err != nil {
				return nil, err
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("unrecognized option '%s'", arg)
		}
		if o.user != "" {
			return nil, fmt.Errorf("extra operand '%s'", arg)
		}
		o.user = arg
	}
	return o, nil
}

// parseLongFlag handles --user, --group, --groups, --name, --real, --zero.
func parseLongFlag(o *opts, arg string) error {
	switch arg {
	case "--user":
		o.flagU = true
	case "--group":
		o.flagG = true
	case "--groups":
		o.flagBigG = true
	case "--name":
		o.flagN = true
	case "--real":
		o.flagR = true
	case "--zero":
		o.flagZero = true
	default:
		return fmt.Errorf("not a long flag")
	}
	return nil
}

// parseShortFlags handles combined short flags like -ugGnrz.
func parseShortFlags(o *opts, flags string) error {
	for _, c := range flags {
		switch c {
		case 'u':
			o.flagU = true
		case 'g':
			o.flagG = true
		case 'G':
			o.flagBigG = true
		case 'n':
			o.flagN = true
		case 'r':
			o.flagR = true
		case 'z':
			o.flagZero = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// validateFlags checks for conflicting flag combinations.
// R2.4: only one of -u, -g, -G may be specified.
// R3.1: -n requires -u, -g, or -G.
// R3.2: -r requires -u or -g.
func validateFlags(o *opts) error {
	selCount := boolCount(o.flagU, o.flagG, o.flagBigG)
	if selCount > 1 {
		return fmt.Errorf("cannot print \"only\" of more than one choice")
	}
	if o.flagN && selCount == 0 {
		return fmt.Errorf("cannot print only names or real IDs in default format")
	}
	if o.flagR && selCount == 0 {
		return fmt.Errorf("cannot print only names or real IDs in default format")
	}
	if o.flagR && o.flagBigG {
		return fmt.Errorf("cannot print only names or real IDs in default format")
	}
	if o.flagZero && selCount == 0 {
		return fmt.Errorf("option --zero not permitted in default format")
	}
	return nil
}

// boolCount returns the number of true values.
func boolCount(vals ...bool) int {
	n := 0
	for _, v := range vals {
		if v {
			n++
		}
	}
	return n
}

// printID dispatches to the appropriate output function.
func printID(o *opts) error {
	if o.flagU {
		return printUID(o)
	}
	if o.flagG {
		return printGID(o)
	}
	if o.flagBigG {
		return printGroups(o)
	}
	return printDefault(o)
}

// printUID prints the effective (or real with -r) user ID or name.
// R2.1: -u prints effective UID. R2.1 + R3.1: -un prints name.
func printUID(o *opts) error {
	uid := os.Geteuid()
	if o.flagR {
		uid = os.Getuid()
	}
	if o.user != "" {
		u, err := lookupUser(o.user)
		if err != nil {
			return err
		}
		uid = parseInt(u.Uid)
	}
	if o.flagN {
		name, err := lookupUserName(uid)
		if err != nil {
			return err
		}
		printValue(name, o.flagZero)
		return nil
	}
	printValue(fmt.Sprintf("%d", uid), o.flagZero)
	return nil
}

// printGID prints the effective (or real with -r) group ID or name.
// R2.2: -g prints effective GID. R2.2 + R3.1: -gn prints name.
func printGID(o *opts) error {
	gid := os.Getegid()
	if o.flagR {
		gid = os.Getgid()
	}
	if o.user != "" {
		u, err := lookupUser(o.user)
		if err != nil {
			return err
		}
		gid = parseInt(u.Gid)
	}
	if o.flagN {
		name, err := lookupGroupName(gid)
		if err != nil {
			return err
		}
		printValue(name, o.flagZero)
		return nil
	}
	printValue(fmt.Sprintf("%d", gid), o.flagZero)
	return nil
}

// printValue prints a value followed by newline or NUL depending on --zero.
func printValue(val string, zero bool) {
	if zero {
		fmt.Printf("%s\x00", val)
	} else {
		fmt.Println(val)
	}
}

// printGroups prints all group IDs (or names with -n).
// R2.3: -G prints all group IDs space-separated.
func printGroups(o *opts) error {
	if o.user != "" {
		return printNamedUserGroups(o)
	}
	return printCurrentGroups(o)
}

// printCurrentGroups prints groups for the current process.
// R3.3: groups are sorted numerically to match GNU id output order.
func printCurrentGroups(o *opts) error {
	gids, err := syscall.Getgroups()
	if err != nil {
		return fmt.Errorf("cannot get groups: %v", err)
	}
	sort.Ints(gids)
	parts := make([]string, 0, len(gids))
	for _, gid := range gids {
		parts = append(parts, formatGID(gid, o.flagN))
	}
	printGroupList(parts, o.flagZero)
	return nil
}

// printNamedUserGroups prints groups for a named user.
// R3.3: groups are sorted numerically to match GNU id output order.
func printNamedUserGroups(o *opts) error {
	u, err := lookupUser(o.user)
	if err != nil {
		return err
	}
	gidStrs, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("failed to get group IDs: %v", err)
	}
	gids := make([]int, 0, len(gidStrs))
	for _, s := range gidStrs {
		gids = append(gids, parseInt(s))
	}
	sort.Ints(gids)
	parts := make([]string, 0, len(gids))
	for _, gid := range gids {
		parts = append(parts, formatGID(gid, o.flagN))
	}
	printGroupList(parts, o.flagZero)
	return nil
}

// printGroupList outputs group entries separated by space or NUL.
func printGroupList(parts []string, zero bool) {
	if zero {
		for _, p := range parts {
			fmt.Printf("%s\x00", p)
		}
	} else {
		fmt.Println(strings.Join(parts, " "))
	}
}

// printDefault prints the full uid=N(name) gid=N(name) groups=... line.
// R1.1: default format with all identity information.
// R1.2: groups list includes all supplementary groups.
func printDefault(o *opts) error {
	if o.user != "" {
		return printDefaultForUser(o.user)
	}
	return printDefaultCurrent()
}

// printDefaultCurrent prints default output for the current process.
func printDefaultCurrent() error {
	uid := os.Geteuid()
	gid := os.Getegid()

	uname, err := lookupUserName(uid)
	if err != nil {
		return err
	}
	gname, err := lookupGroupName(gid)
	if err != nil {
		return err
	}

	gids, err := syscall.Getgroups()
	if err != nil {
		return fmt.Errorf("cannot get groups: %v", err)
	}

	groupParts := formatGroupList(gids)
	fmt.Printf("uid=%d(%s) gid=%d(%s) groups=%s\n", uid, uname, gid, gname, groupParts)
	return nil
}

// printDefaultForUser prints default output for a named user.
func printDefaultForUser(username string) error {
	u, err := lookupUser(username)
	if err != nil {
		return err
	}
	uid := parseInt(u.Uid)
	gid := parseInt(u.Gid)

	gname, err := lookupGroupName(gid)
	if err != nil {
		return err
	}

	gidStrs, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("failed to get group IDs: %v", err)
	}
	gids := make([]int, 0, len(gidStrs))
	for _, s := range gidStrs {
		gids = append(gids, parseInt(s))
	}

	groupParts := formatGroupList(gids)
	fmt.Printf("uid=%d(%s) gid=%d(%s) groups=%s\n", uid, u.Username, gid, gname, groupParts)
	return nil
}

// formatGroupList formats a list of GIDs as N(name),N(name),... for default output.
func formatGroupList(gids []int) string {
	parts := make([]string, 0, len(gids))
	for _, gid := range gids {
		gname, err := lookupGroupName(gid)
		if err != nil {
			parts = append(parts, fmt.Sprintf("%d", gid))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d(%s)", gid, gname))
	}
	return strings.Join(parts, ",")
}

// formatGID formats a single GID as either numeric or name string.
func formatGID(gid int, useName bool) string {
	if useName {
		g, err := user.LookupGroupId(fmt.Sprintf("%d", gid))
		if err != nil {
			return fmt.Sprintf("%d", gid)
		}
		return g.Name
	}
	return fmt.Sprintf("%d", gid)
}

// lookupUser looks up a user by name and returns an error matching GNU id format.
func lookupUser(username string) (*user.User, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("'%s': no such user", username)
	}
	return u, nil
}

// lookupUserName returns the username for a given UID.
func lookupUserName(uid int) (string, error) {
	u, err := user.LookupId(fmt.Sprintf("%d", uid))
	if err != nil {
		return "", fmt.Errorf("cannot find name for user ID %d", uid)
	}
	return u.Username, nil
}

// lookupGroupName returns the group name for a given GID.
func lookupGroupName(gid int) (string, error) {
	g, err := user.LookupGroupId(fmt.Sprintf("%d", gid))
	if err != nil {
		return "", fmt.Errorf("cannot find name for group ID %d", gid)
	}
	return g.Name, nil
}

// parseInt parses a decimal string to int, returning 0 on error.
func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
