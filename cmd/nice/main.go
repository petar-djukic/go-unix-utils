// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd094-nice R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: nice [OPTION] [COMMAND [ARG]...]
Run COMMAND with an adjusted niceness, which affects process scheduling.
With no COMMAND, print the current niceness.  Niceness values range from
-20 (most favorable to the process) to 19 (least favorable to the process).

Mandatory arguments to long options are mandatory for short options too.
  -n, --adjustment=N   add integer N to the niceness (default 10)
      --help           display this help and exit
      --version        output version information and exit
`

const versionText = `nice (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	adjustment, command, cmdArgs, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "nice: %s\n", err)
		os.Exit(125)
	}

	if command == "" {
		prio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nice: cannot get niceness: %s\n", err)
			os.Exit(125)
		}
		fmt.Println(prio)
		os.Exit(0)
	}

	prio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nice: cannot get niceness: %s\n", err)
		os.Exit(125)
	}

	newPrio := min(max(prio+adjustment, -20), 19)

	if err := syscall.Setpriority(syscall.PRIO_PROCESS, 0, newPrio); err != nil {
		fmt.Fprintf(os.Stderr, "nice: cannot set niceness: %s\n", err)
		os.Exit(125)
	}

	binary, err := exec.LookPath(command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nice: '%s': No such file or directory\n", command)
		os.Exit(127)
	}

	argv := append([]string{command}, cmdArgs...)
	execErr := syscall.Exec(binary, argv, os.Environ())
	if execErr != nil {
		fmt.Fprintf(os.Stderr, "nice: '%s': %s\n", command, execErr)
		os.Exit(126)
	}
}

func parseArgs(args []string) (int, string, []string, error) {
	adjustment := 10
	adjustmentSet := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if strings.HasPrefix(arg, "--adjustment=") {
			val := arg[13:]
			n, err := strconv.Atoi(val)
			if err != nil {
				return 0, "", nil, fmt.Errorf("invalid adjustment '%s'", val)
			}
			adjustment = n
			adjustmentSet = true
			i++
			continue
		}
		if arg == "--adjustment" {
			if i+1 >= len(args) {
				return 0, "", nil, fmt.Errorf("option '--adjustment' requires an argument")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, "", nil, fmt.Errorf("invalid adjustment '%s'", args[i+1])
			}
			adjustment = n
			adjustmentSet = true
			i += 2
			continue
		}
		if arg == "--help" {
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		}
		if len(arg) > 1 && arg[0] == '-' && !strings.HasPrefix(arg, "--") {
			if arg[1] == 'n' {
				rest := arg[2:]
				if rest != "" {
					n, err := strconv.Atoi(rest)
					if err != nil {
						return 0, "", nil, fmt.Errorf("invalid adjustment '%s'", rest)
					}
					adjustment = n
					adjustmentSet = true
					i++
					continue
				}
				if i+1 >= len(args) {
					return 0, "", nil, fmt.Errorf("option requires an argument -- 'n'")
				}
				n, err := strconv.Atoi(args[i+1])
				if err != nil {
					return 0, "", nil, fmt.Errorf("invalid adjustment '%s'", args[i+1])
				}
				adjustment = n
				adjustmentSet = true
				i += 2
				continue
			}
			// GNU nice treats -N as --adjustment=N for numeric N
			if n, err := strconv.Atoi(arg[1:]); err == nil {
				adjustment = n
				adjustmentSet = true
				i++
				continue
			}
			break
		}
		break
	}
	_ = adjustmentSet

	if i >= len(args) {
		return adjustment, "", nil, nil
	}
	command := args[i]
	cmdArgs := args[i+1:]
	return adjustment, command, cmdArgs, nil
}
