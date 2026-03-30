// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chroot implements GNU chroot: run a command with a different root directory.
//
// Implements prd100-chroot R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
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

const progName = "chroot"

const exitInternal = 125
const exitCannotExec = 126
const exitNotFound = 127

const versionText = "chroot (go-unix-utils) 0.1"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// config holds parsed command-line options.
type config struct {
	userspec string
	groups   string
	newroot  string
	command  []string
}

// run parses arguments and executes chroot.
func run(args []string) int {
	cfg, code := parseArgs(args)
	if code >= 0 {
		return code
	}
	return execute(cfg)
}

// parseArgs extracts flags and positional arguments.
// Returns config and -1 on success, or config and exit code on early exit.
func parseArgs(args []string) (config, int) {
	var cfg config
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if !strings.HasPrefix(arg, "-") {
			break
		}
		adv, code := handleFlag(arg, args, i, &cfg)
		if code >= 0 {
			return cfg, code
		}
		i += adv
	}
	remaining := args[i:]
	if len(remaining) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		return cfg, exitInternal
	}
	cfg.newroot = remaining[0]
	if len(remaining) > 1 {
		cfg.command = remaining[1:]
	}
	return cfg, -1
}

// handleFlag processes a single flag argument.
// Returns advance count and -1 on success, or 0 and exit code on early exit.
func handleFlag(arg string, args []string, i int, cfg *config) (int, int) {
	switch {
	case arg == "--help":
		printHelp()
		return 0, 0
	case arg == "--version":
		fmt.Println(versionText)
		return 0, 0
	case arg == "--userspec":
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr,
				"%s: option '--userspec' requires an argument\n", progName)
			return 0, exitInternal
		}
		cfg.userspec = args[i+1]
		return 2, -1
	case strings.HasPrefix(arg, "--userspec="):
		cfg.userspec = arg[len("--userspec="):]
		return 1, -1
	case arg == "--groups":
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr,
				"%s: option '--groups' requires an argument\n", progName)
			return 0, exitInternal
		}
		cfg.groups = args[i+1]
		return 2, -1
	case strings.HasPrefix(arg, "--groups="):
		cfg.groups = arg[len("--groups="):]
		return 1, -1
	default:
		fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
		return 0, exitInternal
	}
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: chroot [OPTION] NEWROOT [COMMAND [ARG]...]
  or:  chroot OPTION
Run COMMAND with root directory set to NEWROOT.

      --userspec=USER:GROUP  specify user and group (ID or name) to use
      --groups=G_LIST        specify supplementary groups as g1,g2,..,gN
      --help        display this help and exit
      --version     output version information and exit

If no command is given, run '"${SHELL}" -i' (default: '/bin/sh -i').
`)
}

// execute performs the chroot and exec operations.
func execute(cfg config) int {
	if err := applyChroot(cfg.newroot); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot change root directory to '%s': %s\n",
			progName, cfg.newroot, err)
		return exitInternal
	}
	if err := applyCredentials(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return exitInternal
	}
	cmd := buildCommand(cfg.command)
	return execCommand(cmd)
}

// applyChroot calls chroot(2) and chdir to "/".
func applyChroot(newroot string) error {
	if err := syscall.Chroot(newroot); err != nil {
		return err
	}
	return syscall.Chdir("/")
}

// applyCredentials sets uid, gid, and supplementary groups if specified.
func applyCredentials(cfg config) error {
	if cfg.groups != "" {
		if err := setSupplementaryGroups(cfg.groups); err != nil {
			return fmt.Errorf("invalid --groups: %w", err)
		}
	}
	if cfg.userspec != "" {
		if err := setUserSpec(cfg.userspec); err != nil {
			return fmt.Errorf("invalid --userspec: %w", err)
		}
	}
	return nil
}

// setSupplementaryGroups parses a comma-separated group list and sets them.
// R1.3: --groups=G_LIST sets supplementary groups.
func setSupplementaryGroups(glist string) error {
	parts := strings.Split(glist, ",")
	gids := make([]int, 0, len(parts))
	for _, g := range parts {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		gid, err := resolveGroup(g)
		if err != nil {
			return fmt.Errorf("invalid group '%s': %w", g, err)
		}
		gids = append(gids, gid)
	}
	return syscall.Setgroups(gids)
}

// setUserSpec parses USER[:GROUP] and sets uid and gid.
// R1.2: --userspec=USER:GROUP sets uid/gid.
func setUserSpec(spec string) error {
	uid, gid, hasGroup := parseUserSpec(spec)
	uidNum, err := resolveUser(uid)
	if err != nil {
		return fmt.Errorf("invalid user '%s': %w", uid, err)
	}
	if hasGroup {
		gidNum, err := resolveGroup(gid)
		if err != nil {
			return fmt.Errorf("invalid group '%s': %w", gid, err)
		}
		if err := syscall.Setgid(gidNum); err != nil {
			return fmt.Errorf("cannot set gid to %d: %w", gidNum, err)
		}
	}
	if err := syscall.Setuid(uidNum); err != nil {
		return fmt.Errorf("cannot set uid to %d: %w", uidNum, err)
	}
	return nil
}

// parseUserSpec splits USER[:GROUP] into components.
func parseUserSpec(spec string) (string, string, bool) {
	u, g, ok := strings.Cut(spec, ":")
	if ok {
		return u, g, true
	}
	return spec, "", false
}

// resolveUser converts a username or numeric string to a UID.
func resolveUser(name string) (int, error) {
	if uid, err := strconv.Atoi(name); err == nil {
		return uid, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(u.Uid)
}

// resolveGroup converts a group name or numeric string to a GID.
func resolveGroup(name string) (int, error) {
	if gid, err := strconv.Atoi(name); err == nil {
		return gid, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(g.Gid)
}

// buildCommand returns the command to execute.
// R1.1: default to $SHELL -i or /bin/sh -i.
func buildCommand(args []string) []string {
	if len(args) > 0 {
		return args
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return []string{shell, "-i"}
}

// execCommand replaces the process with the given command.
// R2.1, R2.2: exit codes 125/126/127 for failure modes.
func execCommand(cmd []string) int {
	binary, err := resolveCommand(cmd[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n",
			progName, cmd[0], err)
		return exitNotFound
	}
	err = syscall.Exec(binary, cmd, os.Environ())
	// syscall.Exec only returns on failure
	fmt.Fprintf(os.Stderr, "%s: failed to run command '%s': %s\n",
		progName, cmd[0], err)
	if os.IsNotExist(err) {
		return exitNotFound
	}
	return exitCannotExec
}

// resolveCommand finds the full path of a command.
// If the command contains a slash, it is used as-is.
// Otherwise, it is searched in PATH.
func resolveCommand(name string) (string, error) {
	if strings.Contains(name, "/") {
		if _, err := os.Stat(name); err != nil {
			return "", err
		}
		return name, nil
	}
	return lookPath(name)
}

// lookPath searches PATH for the named executable.
func lookPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		pathEnv = "/usr/bin:/bin"
	}
	for dir := range strings.SplitSeq(pathEnv, ":") {
		if dir == "" {
			dir = "."
		}
		path := dir + "/" + name
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("No such file or directory")
}
