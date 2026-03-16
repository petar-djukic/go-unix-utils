// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd044-uname R1.1-R1.9, R2.1-R2.2, R3.1-R3.2:
// cmd/uname prints system information fields. With no arguments, prints the
// kernel name (equivalent to -s). Supports -s (kernel name), -n (nodename),
// -r (kernel release), -v (kernel version), -m (machine), -p (processor),
// -i (hardware platform), -o (operating system) individually or in combination.
// -a prints all fields in canonical order. Multiple flags print selected fields
// in canonical order separated by spaces.
// Installs SIGPIPE handler per ARCHITECTURE.yaml.
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName returns the program name for error messages, matching GNU behavior
// which uses argv[0].
func progName() string {
	return os.Args[0]
}

// unameFields holds the parsed flag selections for which fields to print.
type unameFields struct {
	sysname          bool // -s: kernel name
	nodename         bool // -n: network node hostname
	release          bool // -r: kernel release
	version          bool // -v: kernel version
	machine          bool // -m: machine hardware name
	processor        bool // -p: processor type
	hardwarePlatform bool // -i: hardware platform
	operatingSystem  bool // -o: operating system
	all              bool // -a: all fields, omitting unknown -p/-i
}

// anySet returns true if at least one field is selected.
func (f *unameFields) anySet() bool {
	return f.sysname || f.nodename || f.release || f.version || f.machine ||
		f.processor || f.hardwarePlatform || f.operatingSystem
}

// setAll sets all field flags to true, implementing -a behavior.
// R2.1: -a prints all fields in canonical order, omitting unknown -p/-i.
func (f *unameFields) setAll() {
	f.all = true
	f.sysname = true
	f.nodename = true
	f.release = true
	f.version = true
	f.machine = true
	f.processor = true
	f.hardwarePlatform = true
	f.operatingSystem = true
}

func main() {
	sys.InstallSIGPIPEHandler()

	fields, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName(), err) //nolint:errcheck // best-effort diagnostic
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName()) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	// R1.1: no flags selected defaults to -s (kernel name).
	if !fields.anySet() {
		fields.sysname = true
	}

	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get system information: %v\n", progName(), err) //nolint:errcheck // best-effort diagnostic
		os.Exit(1)
	}

	// R2.2: collect selected fields in canonical order:
	// sysname, nodename, release, version, machine, processor, hardware-platform, operating-system.
	sysname := byteArrayToString(utsname.Sysname[:])
	var parts []string
	if fields.sysname {
		parts = append(parts, sysname)
	}
	if fields.nodename {
		parts = append(parts, byteArrayToString(utsname.Nodename[:]))
	}
	if fields.release {
		parts = append(parts, byteArrayToString(utsname.Release[:]))
	}
	if fields.version {
		parts = append(parts, byteArrayToString(utsname.Version[:]))
	}
	if fields.machine {
		parts = append(parts, byteArrayToString(utsname.Machine[:]))
	}
	// R1.7: -p processor type — platform-specific, matches GNU uname behavior.
	// R2.1: -a omits processor if "unknown".
	if fields.processor {
		p := processorType()
		if !fields.all || p != "unknown" {
			parts = append(parts, p)
		}
	}
	// R1.8: -i hardware platform — GNU uname outputs "unknown" on most platforms.
	// R2.1: -a omits hardware platform if "unknown".
	if fields.hardwarePlatform {
		hp := "unknown"
		if !fields.all || hp != "unknown" {
			parts = append(parts, hp)
		}
	}
	// R1.9: -o operating system name.
	if fields.operatingSystem {
		parts = append(parts, osName(sysname))
	}

	fmt.Println(strings.Join(parts, " "))
}

// osName returns the operating system name matching GNU uname -o behavior.
// On Linux, GNU uname outputs "GNU/Linux"; on other platforms it outputs the
// kernel name (sysname).
func osName(sysname string) string {
	if sysname == "Linux" {
		return "GNU/Linux"
	}
	return sysname
}

// byteArrayToString converts a null-terminated byte array to a Go string.
func byteArrayToString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// parseArgs parses command-line arguments for uname. Returns the selected
// fields or an error for unknown flags or unexpected operands.
func parseArgs(args []string) (*unameFields, error) {
	fields := &unameFields{}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			// Everything after -- is a positional operand.
			if i+1 < len(args) {
				// R3.1: no positional operands accepted.
				return nil, fmt.Errorf("extra operand '%s'", args[i+1])
			}
			break
		}

		// --help prints usage to stdout and exits 0.
		if arg == "--help" {
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s [OPTION]...\n"+
					"Print certain system information.  With no OPTION, same as -s.\n\n"+
					"  -a, --all                print all information, in the following order,\n"+
					"                             except omit -p and -i if unknown:\n"+
					"  -s, --kernel-name        print the kernel name\n"+
					"  -n, --nodename           print the network node hostname\n"+
					"  -r, --kernel-release     print the kernel release\n"+
					"  -v, --kernel-version     print the kernel version\n"+
					"  -m, --machine            print the machine hardware name\n"+
					"  -p, --processor          print the processor type (non-portable)\n"+
					"  -i, --hardware-platform  print the hardware platform (non-portable)\n"+
					"  -o, --operating-system   print the operating system\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName(),
			)
			os.Exit(0)
		}
		// --version prints version info to stdout and exits 0.
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				progName(), "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}

		// Long options.
		if arg == "--all" {
			fields.setAll()
			continue
		}
		if arg == "--kernel-name" {
			fields.sysname = true
			continue
		}
		if arg == "--nodename" {
			fields.nodename = true
			continue
		}
		if arg == "--kernel-release" {
			fields.release = true
			continue
		}
		if arg == "--kernel-version" {
			fields.version = true
			continue
		}
		if arg == "--machine" {
			fields.machine = true
			continue
		}
		if arg == "--processor" {
			fields.processor = true
			continue
		}
		if arg == "--hardware-platform" {
			fields.hardwarePlatform = true
			continue
		}
		if arg == "--operating-system" {
			fields.operatingSystem = true
			continue
		}

		// Unknown long option.
		if strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("unrecognized option '%s'", arg)
		}

		// Short flag groups (e.g., -snr, -a).
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					// R2.1: -a sets all fields.
					fields.setAll()
				case 's':
					fields.sysname = true
				case 'n':
					fields.nodename = true
				case 'r':
					fields.release = true
				case 'v':
					fields.version = true
				case 'm':
					fields.machine = true
				case 'p':
					fields.processor = true
				case 'i':
					fields.hardwarePlatform = true
				case 'o':
					fields.operatingSystem = true
				default:
					// R3.2: unknown flags produce an error.
					return nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			continue
		}

		// R3.1: no positional operands accepted.
		if arg != "-" {
			return nil, fmt.Errorf("extra operand '%s'", arg)
		}
		// bare "-" is treated as an operand error too.
		return nil, fmt.Errorf("extra operand '%s'", arg)
	}

	return fields, nil
}
