// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd041-id R1.1 (default format uid=N(name) gid=N(name) groups=...),
// R1.2 (supplementary groups in system order), R1.3 (exit 0 on success),
// R2.1 (-u/--user prints effective UID).
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "id"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// config holds parsed command-line options.
type config struct {
	showUser bool   // -u/--user: print only effective UID
	username string // positional USER operand
}

// run parses arguments and dispatches to the appropriate handler.
// Returns the exit code.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		printError(err.Error())
		return 1
	}
	if cfg.username != "" {
		return handleNamedUser(cfg)
	}
	return handleCurrentUser(cfg)
}

// parseArgs parses command-line arguments for id.
// Supports -u/--user, --help, --version, and a single USER operand.
func parseArgs(args []string) (config, error) {
	var cfg config
	pastFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pastFlags = true
			continue
		}
		if !pastFlags && (arg == "--help" || arg == "--version") {
			fmt.Print(helpText(arg))
			os.Exit(0)
		}
		if !pastFlags && (arg == "-u" || arg == "--user") {
			cfg.showUser = true
			continue
		}
		if !pastFlags && strings.HasPrefix(arg, "-") {
			return cfg, fmt.Errorf("unrecognized option '%s'", arg)
		}
		if cfg.username != "" {
			return cfg, fmt.Errorf("extra operand '%s'", arg)
		}
		cfg.username = arg
	}
	return cfg, nil
}

// handleCurrentUser prints identity for the current process user.
func handleCurrentUser(cfg config) int {
	if cfg.showUser {
		fmt.Println(os.Geteuid())
		return 0
	}
	return printDefaultCurrent()
}

// handleNamedUser prints identity for a named user.
func handleNamedUser(cfg config) int {
	u, err := user.Lookup(cfg.username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: '%s': no such user\n",
			programName, cfg.username)
		return 1
	}
	if cfg.showUser {
		fmt.Println(u.Uid)
		return 0
	}
	return printDefaultNamed(u)
}

// printDefaultCurrent prints default identity for the current process.
// Format: uid=N(name) [euid=N(name)] gid=N(name) [egid=N(name)] groups=N(name),...
func printDefaultCurrent() int {
	uid := os.Getuid()
	euid := os.Geteuid()
	gid := os.Getgid()
	egid := os.Getegid()
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot find name for user ID %d\n",
			programName, uid)
		return 1
	}
	parts := buildCurrentParts(u, uid, euid, gid, egid)
	groups, err := os.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to get groups: %v\n",
			programName, err)
		return 1
	}
	parts = append(parts, formatGroupsFromInts(groups))
	fmt.Println(strings.Join(parts, " "))
	return 0
}

// buildCurrentParts builds uid/euid/gid/egid parts for the current process.
func buildCurrentParts(u *user.User, uid, euid, gid, egid int) []string {
	parts := []string{formatIDPart("uid", uid, u.Username)}
	if euid != uid {
		parts = append(parts, formatEuidPart(euid))
	}
	parts = append(parts, formatGidPart("gid", gid))
	if egid != gid {
		parts = append(parts, formatGidPart("egid", egid))
	}
	return parts
}

// printDefaultNamed prints default identity for a named user.
func printDefaultNamed(u *user.User) int {
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	parts := []string{
		formatIDPart("uid", uid, u.Username),
		formatGidPart("gid", gid),
	}
	gids, err := u.GroupIds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to get groups: %v\n",
			programName, err)
		return 1
	}
	parts = append(parts, formatGroupsFromStrings(gids))
	fmt.Println(strings.Join(parts, " "))
	return 0
}

// formatIDPart formats a single "label=N(name)" identity field.
func formatIDPart(label string, id int, name string) string {
	return fmt.Sprintf("%s=%d(%s)", label, id, name)
}

// formatEuidPart formats the euid field with name lookup.
func formatEuidPart(euid int) string {
	name := strconv.Itoa(euid)
	if u, err := user.LookupId(strconv.Itoa(euid)); err == nil {
		name = u.Username
	}
	return formatIDPart("euid", euid, name)
}

// formatGidPart formats a gid or egid field with group name lookup.
func formatGidPart(label string, gid int) string {
	name := lookupGroupName(strconv.Itoa(gid))
	return formatIDPart(label, gid, name)
}

// lookupGroupName returns the group name for a GID string.
// Returns the GID string itself if lookup fails.
func lookupGroupName(gid string) string {
	g, err := user.LookupGroupId(gid)
	if err != nil {
		return gid
	}
	return g.Name
}

// formatGroupsFromInts formats the groups field from integer GIDs.
func formatGroupsFromInts(gids []int) string {
	strs := make([]string, len(gids))
	for i, gid := range gids {
		strs[i] = strconv.Itoa(gid)
	}
	return formatGroupsFromStrings(strs)
}

// formatGroupsFromStrings formats the "groups=N(name),..." field.
func formatGroupsFromStrings(gids []string) string {
	parts := make([]string, len(gids))
	for i, gid := range gids {
		id, _ := strconv.Atoi(gid)
		name := lookupGroupName(gid)
		parts[i] = fmt.Sprintf("%d(%s)", id, name)
	}
	return "groups=" + strings.Join(parts, ",")
}

// helpText returns text for --help or --version.
func helpText(flag string) string {
	if flag == "--version" {
		return "id (go-unix-utils) 1.0\n"
	}
	return `Usage: id [OPTION]... [USER]
Print user and group information for USER or the current user.

  -u, --user     print only the effective user ID
      --help     display this help and exit
      --version  output version information and exit
`
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
