// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd041-id R1.1 (default format uid=N(name) gid=N(name) groups=...),
// R1.2 (supplementary groups in system order), R1.3 (exit 0 on success),
// R2.1 (-u/--user prints effective UID), R2.2 (-g/--group prints effective GID),
// R2.3 (-G/--groups prints all group IDs), R2.4 (mutual exclusivity of -u/-g/-G),
// R3.1 (-n/--name prints name instead of number),
// R3.2 (-r/--real prints real UID/GID instead of effective),
// R3.3 (USER operand queries named user identity).
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
	showUser   bool   // -u/--user: print only effective UID
	showGroup  bool   // -g/--group: print only effective GID
	showGroups bool   // -G/--groups: print all group IDs
	showName   bool   // -n/--name: print name instead of number
	showReal   bool   // -r/--real: print real UID/GID instead of effective
	username   string // positional USER operand
}

// selectionCount returns how many selection flags are active.
func (c config) selectionCount() int {
	count := 0
	if c.showUser {
		count++
	}
	if c.showGroup {
		count++
	}
	if c.showGroups {
		count++
	}
	return count
}

// run parses arguments and dispatches to the appropriate handler.
// Returns the exit code.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		printError(err.Error())
		return 1
	}
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	if cfg.username != "" {
		return handleNamedUser(cfg)
	}
	return handleCurrentUser(cfg)
}

// validateConfig checks for conflicting flag combinations.
// R2.4: only one of -u, -g, -G allowed.
// R3.1: -n requires a selection flag.
// R3.2: -r requires -u or -g.
func validateConfig(cfg config) error {
	if cfg.selectionCount() > 1 {
		return fmt.Errorf("cannot print \"only\" of more than one choice")
	}
	if cfg.showName && cfg.selectionCount() == 0 {
		return fmt.Errorf(
			"printing only names or real IDs requires -u, -g, or -G")
	}
	if cfg.showReal && cfg.selectionCount() == 0 {
		return fmt.Errorf(
			"printing only names or real IDs requires -u, -g, or -G")
	}
	return nil
}

// parseArgs parses command-line arguments for id.
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
		if !pastFlags && strings.HasPrefix(arg, "-") && arg != "-" {
			if err := parseFlag(&cfg, arg); err != nil {
				return cfg, err
			}
			continue
		}
		if cfg.username != "" {
			return cfg, fmt.Errorf("extra operand '%s'", arg)
		}
		cfg.username = arg
	}
	return cfg, nil
}

// parseFlag handles a single flag argument, including combined short flags.
func parseFlag(cfg *config, arg string) error {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(cfg, arg)
	}
	// Parse combined short flags (e.g., -gn, -Gn, -un, -ru).
	for _, ch := range arg[1:] {
		if err := parseShortFlag(cfg, ch); err != nil {
			return err
		}
	}
	return nil
}

// parseLongFlag handles a single long-form flag.
func parseLongFlag(cfg *config, arg string) error {
	switch arg {
	case "--user":
		cfg.showUser = true
	case "--group":
		cfg.showGroup = true
	case "--groups":
		cfg.showGroups = true
	case "--name":
		cfg.showName = true
	case "--real":
		cfg.showReal = true
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}

// parseShortFlag handles a single short flag character.
func parseShortFlag(cfg *config, ch rune) error {
	switch ch {
	case 'u':
		cfg.showUser = true
	case 'g':
		cfg.showGroup = true
	case 'G':
		cfg.showGroups = true
	case 'n':
		cfg.showName = true
	case 'r':
		cfg.showReal = true
	default:
		return fmt.Errorf("unrecognized option '-%c'", ch)
	}
	return nil
}

// handleCurrentUser prints identity for the current process user.
func handleCurrentUser(cfg config) int {
	if cfg.showUser {
		return printCurrentUID(cfg.showName, cfg.showReal)
	}
	if cfg.showGroup {
		return printCurrentGID(cfg.showName, cfg.showReal)
	}
	if cfg.showGroups {
		return printCurrentGroups(cfg.showName)
	}
	return printDefaultCurrent()
}

// printCurrentUID prints the current user's UID.
// R3.2: uses real UID if showReal is true, effective UID otherwise.
func printCurrentUID(showName, showReal bool) int {
	uid := os.Geteuid()
	if showReal {
		uid = os.Getuid()
	}
	if showName {
		u, err := user.LookupId(strconv.Itoa(uid))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot find name for user ID %d\n",
				programName, uid)
			return 1
		}
		fmt.Println(u.Username)
		return 0
	}
	fmt.Println(uid)
	return 0
}

// printCurrentGID prints the current user's GID.
// R3.2: uses real GID if showReal is true, effective GID otherwise.
func printCurrentGID(showName, showReal bool) int {
	gid := os.Getegid()
	if showReal {
		gid = os.Getgid()
	}
	if showName {
		fmt.Println(lookupGroupName(strconv.Itoa(gid)))
		return 0
	}
	fmt.Println(gid)
	return 0
}

// printCurrentGroups prints all group IDs for the current user.
func printCurrentGroups(showName bool) int {
	groups, err := os.Getgroups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to get groups: %v\n",
			programName, err)
		return 1
	}
	printGroupList(groups, showName)
	return 0
}

// printGroupList prints group IDs or names space-separated.
func printGroupList(gids []int, showName bool) {
	strs := make([]string, len(gids))
	for i, gid := range gids {
		if showName {
			strs[i] = lookupGroupName(strconv.Itoa(gid))
		} else {
			strs[i] = strconv.Itoa(gid)
		}
	}
	fmt.Println(strings.Join(strs, " "))
}

// handleNamedUser prints identity for a named user.
// R3.3: queries named user's identity from the system.
func handleNamedUser(cfg config) int {
	u, err := user.Lookup(cfg.username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: '%s': no such user\n",
			programName, cfg.username)
		return 1
	}
	if cfg.showUser {
		return printNamedUID(u, cfg.showName)
	}
	if cfg.showGroup {
		return printNamedGID(u, cfg.showName)
	}
	if cfg.showGroups {
		return printNamedGroups(u, cfg.showName)
	}
	return printDefaultNamed(u)
}

// printNamedUID prints a named user's UID.
func printNamedUID(u *user.User, showName bool) int {
	if showName {
		fmt.Println(u.Username)
		return 0
	}
	fmt.Println(u.Uid)
	return 0
}

// printNamedGID prints a named user's primary GID.
func printNamedGID(u *user.User, showName bool) int {
	if showName {
		fmt.Println(lookupGroupName(u.Gid))
		return 0
	}
	fmt.Println(u.Gid)
	return 0
}

// printNamedGroups prints all group IDs for a named user.
func printNamedGroups(u *user.User, showName bool) int {
	gids, err := u.GroupIds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to get groups: %v\n",
			programName, err)
		return 1
	}
	strs := make([]string, len(gids))
	for i, gid := range gids {
		if showName {
			strs[i] = lookupGroupName(gid)
		} else {
			strs[i] = gid
		}
	}
	fmt.Println(strings.Join(strs, " "))
	return 0
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
  -g, --group    print only the effective group ID
  -G, --groups   print all group IDs
  -n, --name     print a name instead of a number, for -ugG
  -r, --real     print the real ID instead of the effective ID, with -ug
      --help     display this help and exit
      --version  output version information and exit
`
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
