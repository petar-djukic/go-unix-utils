// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd044-uname R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R1.7, R1.8, R1.9, R2.1, R2.2, R3.1, R3.2
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and help messages.
const programName = "uname"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// Parse flags manually to match GNU getopt behavior (combined flags like -snrvm).
	var flagS, flagN, flagR, flagV, flagM, flagP, flagI, flagO, flagA bool
	var positional []string

	for _, arg := range args {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags: parse each character.
			for _, ch := range arg[1:] {
				switch ch {
				case 's':
					flagS = true
				case 'n':
					flagN = true
				case 'r':
					flagR = true
				case 'v':
					// R1.5: kernel version string.
					flagV = true
				case 'm':
					// R1.6: machine hardware name.
					flagM = true
				case 'p':
					// R1.7: processor type.
					flagP = true
				case 'i':
					// R1.8: hardware platform.
					flagI = true
				case 'o':
					// R1.9: operating system name.
					flagO = true
				case 'a':
					// R2.1: all fields.
					flagA = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\nTry '%s --help' for more information.\n", programName, ch, programName)
					os.Exit(1)
				}
			}
		} else if arg == "--help" {
			fmt.Print(helpText)
			return
		} else if arg == "--version" {
			fmt.Println("uname (go-unix-utils) 0.1")
			return
		} else if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\nTry '%s --help' for more information.\n", programName, arg, programName)
			os.Exit(1)
		} else {
			positional = append(positional, arg)
		}
	}

	// R3.1: reject positional operands.
	if len(positional) > 0 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\nTry '%s --help' for more information.\n", programName, positional[0], programName)
		os.Exit(1)
	}

	// R2.1: -a sets all individual flags.
	if flagA {
		flagS = true
		flagN = true
		flagR = true
		flagV = true
		flagM = true
		flagP = true
		flagI = true
		flagO = true
	}

	// R1.1: no flags means print kernel name (equivalent to -s).
	if !flagS && !flagN && !flagR && !flagV && !flagM && !flagP && !flagI && !flagO {
		flagS = true
	}

	// R2.1: GNU uname -a omits -p and -i when their values are "unknown".
	omitUnknownP := flagA && processorType() == "unknown"
	omitUnknownI := flagA && hardwarePlatform() == "unknown"

	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}

	// R2.2: print requested fields in canonical order, space-separated.
	var fields []string
	if flagS {
		// R1.2: kernel name.
		fields = append(fields, bytesToString(utsname.Sysname))
	}
	if flagN {
		// R1.3: network node hostname.
		fields = append(fields, bytesToString(utsname.Nodename))
	}
	if flagR {
		// R1.4: kernel release string.
		fields = append(fields, bytesToString(utsname.Release))
	}
	if flagV {
		// R1.5: kernel version string.
		fields = append(fields, bytesToString(utsname.Version))
	}
	if flagM {
		// R1.6: machine hardware name.
		fields = append(fields, bytesToString(utsname.Machine))
	}
	if flagP && !omitUnknownP {
		// R1.7: processor type — "unknown" if not determinable.
		fields = append(fields, processorType())
	}
	if flagI && !omitUnknownI {
		// R1.8: hardware platform — "unknown" if not determinable.
		fields = append(fields, hardwarePlatform())
	}
	if flagO {
		// R1.9: operating system name.
		fields = append(fields, operatingSystem())
	}

	fmt.Println(strings.Join(fields, " "))
}

// processorType returns the processor type string matching GNU coreutils behavior.
// On Darwin, GNU coreutils embeds the processor from the configure-time uname -p:
// "arm" on Apple Silicon, "i386" on Intel. On Linux, it defaults to "unknown".
func processorType() string {
	if runtime.GOOS == "darwin" {
		switch runtime.GOARCH {
		case "arm64":
			return "arm"
		case "amd64":
			return "i386"
		}
	}
	return "unknown"
}

// hardwarePlatform returns the hardware platform string matching GNU coreutils behavior.
// R1.8: GNU coreutils returns "unknown" for hardware platform on all platforms.
func hardwarePlatform() string {
	return "unknown"
}

// operatingSystem returns the operating system name matching GNU coreutils behavior.
// R1.9: On Darwin, GNU coreutils returns "Darwin". On Linux, it returns "GNU/Linux".
func operatingSystem() string {
	if runtime.GOOS == "linux" {
		return "GNU/Linux"
	}
	return "Darwin"
}

// bytesToString converts a null-terminated byte array to a Go string.
func bytesToString(field [256]byte) string {
	for i, b := range field {
		if b == 0 {
			return string(field[:i])
		}
	}
	return string(field[:])
}

// helpText is the usage message printed by --help.
const helpText = `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -a, --all                print all information, in the following order,
                             except omit -p and -i if unknown:
  -s, --kernel-name        print the kernel name
  -n, --nodename           print the network node hostname
  -r, --kernel-release     print the kernel release
  -v, --kernel-version     print the kernel version
  -m, --machine            print the machine hardware name
  -p, --processor          print the processor type (non-portable)
  -i, --hardware-platform  print the hardware platform (non-portable)
  -o, --operating-system   print the operating system
      --help     display this help and exit
      --version  output version information and exit
`
