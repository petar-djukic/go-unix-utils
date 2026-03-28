// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chroot implements GNU chroot: run a command with a different root directory.
// Implements prd100-chroot R1.1-R1.3, R2.1-R2.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName     = "chroot"
	exitInternal = 125
	exitNoExec   = 126
	exitNotFound = 127
)

func main() {
	sys.InstallSIGPIPEHandler()
	opts, newroot, cmd, args := parseArgs(os.Args[1:])
	creds, err := resolveCredentials(opts)
	if err != nil {
		exitWithError(err.Error())
	}
	runChroot(newroot, cmd, args, creds)
}

// options holds parsed --userspec and --groups values.
type options struct {
	userspec string
	groups   string
}

// credentials holds resolved numeric IDs for user/group switching.
// Resolution happens before chroot; application happens after.
type credentials struct {
	hasUser  bool
	uid      int
	hasGroup bool
	gid      int
	suppGids []int
}

// parseArgs extracts options, NEWROOT, COMMAND, and ARGs from arguments.
// R1.1: NEWROOT is required; COMMAND defaults to ${SHELL} -i or /bin/sh -i.
func parseArgs(args []string) (options, string, string, []string) {
	var opts options
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			i++
			break
		}
		if !strings.HasPrefix(args[i], "-") {
			break
		}
		consumed, err := parseFlag(args[i:], &opts)
		if err != nil {
			exitWithError(err.Error())
		}
		i += consumed
	}
	if i >= len(args) {
		exitWithError("missing operand")
	}
	newroot := args[i]
	i++
	cmd, cmdArgs := defaultCommand(args[i:])
	return opts, newroot, cmd, cmdArgs
}

// parseFlag parses one flag from args[0] into opts.
// Returns the number of arguments consumed.
func parseFlag(args []string, opts *options) (int, error) {
	arg := args[0]
	switch {
	case arg == "--help":
		printHelp()
		os.Exit(0)
	case arg == "--version":
		printVersion()
		os.Exit(0)
	case strings.HasPrefix(arg, "--userspec="):
		opts.userspec = arg[len("--userspec="):]
		return 1, nil
	case strings.HasPrefix(arg, "--groups="):
		opts.groups = arg[len("--groups="):]
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil // unreachable
}

// defaultCommand returns the command and args to execute.
// R1.1: defaults to ${SHELL} -i, falling back to /bin/sh -i.
func defaultCommand(args []string) (string, []string) {
	if len(args) > 0 {
		return args[0], args[1:]
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, []string{"-i"}
}

// resolveCredentials resolves --userspec and --groups to numeric IDs.
// R1.2, R1.3: resolution uses the host filesystem before chroot.
func resolveCredentials(opts options) (credentials, error) {
	var creds credentials
	if opts.groups != "" {
		gids, err := resolveGroupList(opts.groups)
		if err != nil {
			return creds, err
		}
		creds.suppGids = gids
	}
	if opts.userspec != "" {
		return resolveUserSpec(creds, opts.userspec)
	}
	return creds, nil
}

// resolveUserSpec parses USER[:GROUP] and resolves to numeric IDs.
// R1.2: USER and GROUP may be names or numeric IDs.
func resolveUserSpec(creds credentials, spec string) (credentials, error) {
	parts := strings.SplitN(spec, ":", 2)
	uid, err := resolveUser(parts[0])
	if err != nil {
		return creds, fmt.Errorf("invalid user '%s': %w", parts[0], err)
	}
	creds.hasUser = true
	creds.uid = uid
	if len(parts) == 2 && parts[1] != "" {
		gid, err := resolveGroup(parts[1])
		if err != nil {
			return creds, fmt.Errorf("invalid group '%s': %w", parts[1], err)
		}
		creds.hasGroup = true
		creds.gid = gid
	}
	return creds, nil
}

// resolveGroupList parses a comma-separated group list to numeric GIDs.
// R1.3: G_LIST is comma-separated group names or GIDs.
func resolveGroupList(glist string) ([]int, error) {
	parts := strings.Split(glist, ",")
	gids := make([]int, 0, len(parts))
	for _, g := range parts {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		gid, err := resolveGroup(g)
		if err != nil {
			return nil, fmt.Errorf("invalid group '%s': %w", g, err)
		}
		gids = append(gids, gid)
	}
	return gids, nil
}

// runChroot performs the chroot, chdir, credential switch, and exec sequence.
// R1.1: chroot(2) then chdir("/") then exec COMMAND.
// R2.2: exit 125 on chroot/chdir/credential failure.
func runChroot(newroot, cmd string, args []string, creds credentials) {
	if err := syscall.Chroot(newroot); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot change root directory to '%s': %s\n",
			progName, newroot, err)
		os.Exit(exitInternal)
	}
	if err := syscall.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot chdir to '/': %s\n", progName, err)
		os.Exit(exitInternal)
	}
	if err := applyCredentials(creds); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(exitInternal)
	}
	execCommand(cmd, args)
}

// applyCredentials sets supplementary groups, GID, and UID after chroot.
// R1.2, R1.3: credentials applied after chroot but before exec.
func applyCredentials(creds credentials) error {
	if creds.suppGids != nil {
		if err := syscall.Setgroups(creds.suppGids); err != nil {
			return fmt.Errorf("cannot set supplementary groups: %w", err)
		}
	}
	if creds.hasGroup {
		if err := syscall.Setgid(creds.gid); err != nil {
			return fmt.Errorf("cannot set gid to %d: %w", creds.gid, err)
		}
	}
	if creds.hasUser {
		if err := syscall.Setuid(creds.uid); err != nil {
			return fmt.Errorf("cannot set uid to %d: %w", creds.uid, err)
		}
	}
	return nil
}

// execCommand replaces the process with the given command.
// R2.1: exit with COMMAND's exit status (syscall.Exec replaces the process).
// R2.2: exit 126 if found but not executable, 127 if not found.
func execCommand(cmd string, args []string) {
	path := resolveCommand(cmd)
	argv := append([]string{cmd}, args...)
	err := syscall.Exec(path, argv, os.Environ())
	// syscall.Exec only returns on error.
	fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n", progName, cmd, err)
	if errors.Is(err, syscall.ENOENT) {
		os.Exit(exitNotFound)
	}
	os.Exit(exitNoExec)
}

// resolveCommand finds the full path for a command.
// R2.2: exits 127 if not found, 126 if not executable.
func resolveCommand(cmd string) string {
	if strings.Contains(cmd, "/") {
		return resolvePathCommand(cmd)
	}
	path, err := lookPath(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n", progName, cmd, err)
		os.Exit(exitNotFound)
	}
	return path
}

// resolvePathCommand handles commands containing a slash.
// R2.2: exits 127 if not found, 126 if found but not executable.
func resolvePathCommand(cmd string) string {
	info, err := os.Stat(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n", progName, cmd, err)
		os.Exit(exitNotFound)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': Permission denied\n",
			progName, cmd)
		os.Exit(exitNoExec)
	}
	return cmd
}

// lookPath searches PATH for the named executable.
func lookPath(cmd string) (string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("no such file or directory")
	}
	for _, dir := range strings.Split(pathEnv, ":") {
		p := dir + "/" + cmd
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("no such file or directory")
}

// resolveUser looks up a user by name or numeric UID.
func resolveUser(name string) (int, error) {
	u, err := user.Lookup(name)
	if err == nil {
		return strconv.Atoi(u.Uid)
	}
	uid, err := strconv.Atoi(name)
	if err != nil {
		return 0, fmt.Errorf("no such user")
	}
	return uid, nil
}

// resolveGroup looks up a group by name or numeric GID.
func resolveGroup(name string) (int, error) {
	g, err := user.LookupGroup(name)
	if err == nil {
		return strconv.Atoi(g.Gid)
	}
	gid, err := strconv.Atoi(name)
	if err != nil {
		return 0, fmt.Errorf("no such group")
	}
	return gid, nil
}

// exitWithError prints an error and exits 125.
// R2.2: exit 125 when chroot itself fails.
func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	os.Exit(exitInternal)
}

// printHelp outputs usage information to stdout.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION] NEWROOT [COMMAND [ARG]...]\n", progName)
	fmt.Println("Run COMMAND with root directory set to NEWROOT.")
	fmt.Println()
	fmt.Println("  --userspec=USER:GROUP  specify user and group (ID or name)")
	fmt.Println("  --groups=G_LIST        specify supplementary groups")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
	fmt.Println()
	fmt.Println("If no command is given, run '${SHELL} -i' (default: '/bin/sh -i').")
}

// printVersion outputs version information to stdout.
func printVersion() {
	fmt.Printf("%s (go-unix-utils)\n", progName)
}
