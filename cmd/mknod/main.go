// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd093-mknod: Make Block or Character Special Files.
// Covers R1.1-R1.3 (special file creation, mode setting, error handling),
// R2.1-R2.3 (exit codes, SIGPIPE handling).
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, exit := parseArgs(os.Args[1:])
	if exit >= 0 {
		os.Exit(exit)
	}

	os.Exit(run(cfg))
}

// config holds parsed flag and argument state.
type config struct {
	mode    os.FileMode
	modeSet bool
	name    string
	typ     string
	major   uint32
	minor   uint32
}

// run creates the special file and returns the exit code.
// R2.1: exits 0 on success, R2.2: exits 1 on failure.
func run(cfg config) int {
	mode := defaultMode(cfg)
	if cfg.typ == "p" {
		return createFIFO(cfg.name, mode)
	}
	return createDevice(cfg, mode)
}

// defaultMode returns the permission bits for the file type.
// R1.2: 0666 for FIFOs, 0660 for device files (before umask).
func defaultMode(cfg config) os.FileMode {
	if cfg.modeSet {
		return cfg.mode
	}
	if cfg.typ == "p" {
		return 0o666
	}
	return 0o660
}

// createFIFO creates a FIFO (named pipe) using syscall.Mkfifo.
// D3: type p uses Mkfifo, not Mknod.
func createFIFO(name string, mode os.FileMode) int {
	if err := syscall.Mkfifo(name, uint32(mode)); err != nil {
		printError(name, err)
		return 1
	}
	return 0
}

// createDevice creates a block or character special file.
// D2: uses syscall.Mknod with unix.Mkdev for major/minor encoding.
func createDevice(cfg config, mode os.FileMode) int {
	sysMode := deviceSysMode(cfg.typ, mode)
	dev := int(unix.Mkdev(cfg.major, cfg.minor))
	if err := syscall.Mknod(cfg.name, sysMode, dev); err != nil {
		printError(cfg.name, err)
		return 1
	}
	return 0
}

// deviceSysMode combines file type bits with permission bits.
func deviceSysMode(typ string, mode os.FileMode) uint32 {
	var typeBits uint32
	if typ == "b" {
		typeBits = syscall.S_IFBLK
	} else {
		typeBits = syscall.S_IFCHR
	}
	return typeBits | uint32(mode)
}

// printError formats an error in GNU mknod style.
func printError(name string, err error) {
	reason := capitalizeFirst(err.Error())
	fmt.Fprintf(os.Stderr, "mknod: %s: %s\n", name, reason)
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseArgs processes flags and positional arguments.
// exit is -1 when processing should continue; >= 0 for early termination.
func parseArgs(args []string) (cfg config, exit int) {
	exit = -1
	i := 0
	i, exit = parseFlags(args, &cfg)
	if exit >= 0 {
		return
	}
	positional := args[i:]
	return parsePositional(cfg, positional)
}

// parseFlags processes flag arguments and returns the index of
// the first positional argument.
func parseFlags(args []string, cfg *config) (int, int) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return i + 1, -1
		case arg == "--help":
			return 0, printHelp()
		case arg == "--version":
			return 0, printVersion()
		case arg == "-m":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr,
					"mknod: option requires an argument -- 'm'")
				return 0, 1
			}
			if exit := setMode(cfg, args[i]); exit >= 0 {
				return 0, exit
			}
		case strings.HasPrefix(arg, "--mode="):
			val := strings.TrimPrefix(arg, "--mode=")
			if exit := setMode(cfg, val); exit >= 0 {
				return 0, exit
			}
		case strings.HasPrefix(arg, "-m"):
			val := arg[2:]
			if exit := setMode(cfg, val); exit >= 0 {
				return 0, exit
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			fmt.Fprintf(os.Stderr,
				"mknod: unrecognized option '%s'\n", arg)
			return 0, 1
		default:
			return i, -1
		}
	}
	return len(args), -1
}

// setMode parses and sets the mode on config, returning exit >= 0 on error.
func setMode(cfg *config, val string) int {
	mode, err := parseMode(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mknod: invalid mode '%s'\n", val)
		return 1
	}
	cfg.mode = mode
	cfg.modeSet = true
	return -1
}

// parsePositional validates positional args: NAME TYPE [MAJOR MINOR].
// R1.1: TYPE b/c/u require MAJOR MINOR; TYPE p must not have them.
// R1.3: exits 1 with error for invalid arguments.
func parsePositional(cfg config, args []string) (config, int) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "mknod: missing operand")
		printTryHelp()
		return cfg, 1
	}
	if len(args) == 1 {
		fmt.Fprintf(os.Stderr,
			"mknod: missing operand after '%s'\n", args[0])
		printTryHelp()
		return cfg, 1
	}

	cfg.name = args[0]
	cfg.typ = args[1]

	switch cfg.typ {
	case "p":
		return parseFIFOArgs(cfg, args)
	case "b", "c", "u":
		return parseDeviceArgs(cfg, args)
	default:
		fmt.Fprintf(os.Stderr,
			"mknod: invalid device type '%s'\n", cfg.typ)
		printTryHelp()
		return cfg, 1
	}
}

// parseFIFOArgs validates that FIFO type has no extra arguments.
func parseFIFOArgs(cfg config, args []string) (config, int) {
	if len(args) > 2 {
		fmt.Fprintln(os.Stderr,
			"mknod: Fifos do not have major and minor device numbers.")
		printTryHelp()
		return cfg, 1
	}
	return cfg, -1
}

// parseDeviceArgs validates and parses MAJOR MINOR for device types.
func parseDeviceArgs(cfg config, args []string) (config, int) {
	if len(args) < 4 {
		fmt.Fprintf(os.Stderr,
			"mknod: missing operand after '%s'\n", args[len(args)-1])
		printTryHelp()
		return cfg, 1
	}
	if len(args) > 4 {
		fmt.Fprintf(os.Stderr,
			"mknod: extra operand '%s'\n", args[4])
		printTryHelp()
		return cfg, 1
	}
	major, err := parseDevNum(args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"mknod: invalid major device number '%s'\n", args[2])
		return cfg, 1
	}
	minor, err := parseDevNum(args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"mknod: invalid minor device number '%s'\n", args[3])
		return cfg, 1
	}
	cfg.major = major
	cfg.minor = minor
	return cfg, -1
}

// parseDevNum parses a device number string into uint32.
func parseDevNum(s string) (uint32, error) {
	val, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(val), nil
}

// parseMode parses an octal permission string.
// R1.2: MODE is an octal value like "0666" or "600".
func parseMode(s string) (os.FileMode, error) {
	val, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(val), nil
}

// printTryHelp prints the standard "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintln(os.Stderr, "Try 'mknod --help' for more information.")
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: mknod [OPTION]... NAME TYPE [MAJOR MINOR]
Create the special file NAME of the given TYPE.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file permission bits to MODE, not a=rw - umask

      --help     display this help and exit
      --version  output version information and exit

Both MAJOR and MINOR must be specified when TYPE is b, c, or u, and they
must be omitted when TYPE is p.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout,
		"mknod (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
