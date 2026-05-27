// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd100-chroot R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: chroot [OPTION] NEWROOT [COMMAND [ARG]...]
  or:  chroot OPTION
Run COMMAND with root directory set to NEWROOT.

      --groups=G_LIST
         specify supplementary groups as g1,g2,..,gN
      --userspec=USER:GROUP
         specify user and group (ID or name) to use
      --help     display this help and exit
      --version  output version information and exit

If no command is given, run '"$SHELL" -i' (default: '/bin/sh -i').
`

const versionText = `chroot (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	userspec, groups, newroot, command, cmdArgs := parseArgs(os.Args[1:])

	uid, gid := resolveUserspec(userspec)
	suppGids := resolveGroups(groups)

	if err := syscall.Chroot(newroot); err != nil {
		fmt.Fprintf(os.Stderr, "chroot: cannot change root directory to '%s': %s\n", newroot, err)
		os.Exit(125)
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "chroot: cannot chdir to '/': %s\n", err)
		os.Exit(125)
	}

	if suppGids != nil {
		if err := syscall.Setgroups(suppGids); err != nil {
			fmt.Fprintf(os.Stderr, "chroot: failed to set supplementary groups: %s\n", err)
			os.Exit(125)
		}
	}

	if gid >= 0 {
		if err := syscall.Setgid(gid); err != nil {
			fmt.Fprintf(os.Stderr, "chroot: failed to set group-id: %s\n", err)
			os.Exit(125)
		}
	}

	if uid >= 0 {
		if err := syscall.Setuid(uid); err != nil {
			fmt.Fprintf(os.Stderr, "chroot: failed to set user-id: %s\n", err)
			os.Exit(125)
		}
	}

	binary, err := exec.LookPath(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chroot: failed to run command '%s': %s\n", command, err)
		os.Exit(127)
	}

	argv := append([]string{command}, cmdArgs...)
	execErr := syscall.Exec(binary, argv, os.Environ())
	if execErr != nil {
		fmt.Fprintf(os.Stderr, "chroot: failed to run command '%s': %s\n", command, execErr)
		os.Exit(126)
	}
}

func parseArgs(args []string) (string, string, string, string, []string) {
	var userspec, groups string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "--help" {
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "--userspec=") {
			userspec = arg[len("--userspec="):]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--groups=") {
			groups = arg[len("--groups="):]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "chroot: unrecognized option '%s'\n", arg)
			fmt.Fprintf(os.Stderr, "Try 'chroot --help' for more information.\n")
			os.Exit(125)
		}
		break
	}

	if i >= len(args) {
		fmt.Fprintf(os.Stderr, "chroot: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'chroot --help' for more information.\n")
		os.Exit(125)
	}

	newroot := args[i]
	i++

	var command string
	var cmdArgs []string
	if i < len(args) {
		command = args[i]
		cmdArgs = args[i+1:]
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		command = shell
		cmdArgs = []string{"-i"}
	}

	return userspec, groups, newroot, command, cmdArgs
}

func resolveUserspec(spec string) (int, int) {
	if spec == "" {
		return -1, -1
	}

	userPart, groupPart, hasColon := strings.Cut(spec, ":")
	uid := -1
	gid := -1

	if userPart != "" {
		uid = lookupUID(userPart)
	}
	if hasColon && groupPart != "" {
		gid = lookupGID(groupPart)
	}

	return uid, gid
}

func resolveGroups(groupList string) []int {
	if groupList == "" {
		return nil
	}

	parts := strings.Split(groupList, ",")
	gids := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		gids = append(gids, lookupGID(p))
	}
	return gids
}

func lookupUID(name string) int {
	if uid, err := strconv.Atoi(name); err == nil {
		return uid
	}
	u, err := user.Lookup(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chroot: invalid user: '%s'\n", name)
		os.Exit(125)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chroot: invalid user: '%s'\n", name)
		os.Exit(125)
	}
	return uid
}

func lookupGID(name string) int {
	if gid, err := strconv.Atoi(name); err == nil {
		return gid
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chroot: invalid group: '%s'\n", name)
		os.Exit(125)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chroot: invalid group: '%s'\n", name)
		os.Exit(125)
	}
	return gid
}
