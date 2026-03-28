// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd099-shred: Overwrite a File to Hide Its Contents.
// Covers R1.1-R1.4 (overwrite, iterations, zero pass, remove),
// R2.1-R2.4 (verbose, exact, size, multiple files, error handling),
// R3.1-R3.3 (exit codes, SIGPIPE).
package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "shred"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds parsed command-line options.
type config struct {
	iterations  int
	addZero     bool
	remove      bool
	verbose     bool
	exact       bool
	sizeStr     string
	showHelp    bool
	showVersion bool
	files       []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and processes files. Returns exit code.
// R3.1: exit 0 on success. R3.2: exit 1 on error.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		printTryHelp()
		return 1
	}
	if cfg.showHelp {
		return printHelp()
	}
	if cfg.showVersion {
		return printVersion()
	}
	if len(cfg.files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", progName)
		printTryHelp()
		return 1
	}
	return shredFiles(cfg)
}

// shredFiles processes each file. R2.3: multiple files. R2.4: continue on error.
func shredFiles(cfg config) int {
	size, err := resolveSize(cfg.sizeStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	exitCode := 0
	for _, f := range cfg.files {
		if err := shredOne(cfg, f, size); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// resolveSize parses the --size value, returning 0 when unset.
// R2.2: supports K/M/G/T suffixes via sizeparse.
func resolveSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, nil
	}
	n, err := sizeparse.Parse(sizeStr)
	if err != nil {
		return 0, fmt.Errorf("invalid file size: %q", sizeStr)
	}
	if n <= 0 {
		return 0, fmt.Errorf("invalid file size: %q", sizeStr)
	}
	return n, nil
}

// shredOne overwrites and optionally removes a single file.
// R1.1: overwrite with random data. R1.4: --remove unlinks.
func shredOne(cfg config, path string, size int64) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %v", path, unwrapErr(err))
	}
	fileSize := computeWriteSize(fi, size, cfg.exact)
	// D2: total passes includes the zero pass when --zero is set.
	totalPasses := cfg.iterations
	if cfg.addZero {
		totalPasses++
	}
	if err := overwritePasses(cfg, path, fileSize, totalPasses); err != nil {
		return err
	}
	if cfg.addZero {
		if err := zeroPass(cfg, path, fileSize, totalPasses); err != nil {
			return err
		}
	}
	if cfg.remove {
		return removeFile(cfg, path, fileSize)
	}
	return nil
}

// computeWriteSize determines the byte count to overwrite.
// R2.2: --exact skips block rounding. Without --exact, rounds up
// to the next full block boundary.
func computeWriteSize(fi os.FileInfo, explicitSize int64, exact bool) int64 {
	if explicitSize > 0 {
		return explicitSize
	}
	fileSize := fi.Size()
	if exact || fileSize == 0 {
		return fileSize
	}
	return roundUpToBlock(fileSize)
}

// roundUpToBlock rounds n up to the next multiple of blockSize.
func roundUpToBlock(n int64) int64 {
	remainder := n % blockSize
	if remainder == 0 {
		return n
	}
	return n + blockSize - remainder
}

// overwritePasses performs N random overwrite passes. R1.1, R1.2.
// R2.1: verbose prints pass progress with total including zero pass.
func overwritePasses(cfg config, path string, size int64, totalPasses int) error {
	for i := 0; i < cfg.iterations; i++ {
		if cfg.verbose {
			printProgress(path, i+1, totalPasses, "random")
		}
		if err := writePass(path, size, false); err != nil {
			return err
		}
	}
	return nil
}

// zeroPass performs a final zero-overwrite pass. R1.3.
// R2.1: verbose prints pass N+1/N+1 with (000000).
func zeroPass(cfg config, path string, size int64, totalPasses int) error {
	if cfg.verbose {
		printProgress(path, totalPasses, totalPasses, "000000")
	}
	return writePass(path, size, true)
}

// writePass overwrites a file with random or zero data and syncs.
func writePass(path string, size int64, zero bool) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("%s: %v", path, unwrapErr(err))
	}
	defer f.Close() // best-effort close on error path
	if err := writeBlocks(f, size, zero); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	return nil
}

const blockSize = 64 * 1024 // 64 KiB write blocks

// writeBlocks writes data in blockSize chunks until size bytes written.
func writeBlocks(f *os.File, size int64, zero bool) error {
	buf := make([]byte, blockSize)
	if zero {
		// buf is already zeroed by make
		return writeChunks(f, buf, size)
	}
	return writeRandomChunks(f, buf, size)
}

// writeChunks writes a pre-filled buffer in chunks up to total bytes.
func writeChunks(f *os.File, buf []byte, total int64) error {
	remaining := total
	for remaining > 0 {
		n := min(int64(len(buf)), remaining)
		written, err := f.Write(buf[:n])
		if err != nil {
			return err
		}
		remaining -= int64(written)
	}
	return nil
}

// writeRandomChunks fills buf with random data and writes in chunks.
func writeRandomChunks(f *os.File, buf []byte, total int64) error {
	remaining := total
	for remaining > 0 {
		n := min(int64(len(buf)), remaining)
		if _, err := rand.Read(buf[:n]); err != nil {
			return fmt.Errorf("reading random data: %w", err)
		}
		written, err := f.Write(buf[:n])
		if err != nil {
			return err
		}
		remaining -= int64(written)
	}
	return nil
}

// removeFile truncates, renames progressively, then unlinks. R1.4.
func removeFile(cfg config, path string, size int64) error {
	if err := truncateSteps(path, size); err != nil {
		return err
	}
	renamed, err := renameSteps(cfg, path)
	if err != nil {
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(os.Stderr, "%s: %s: removed\n", progName, path)
	}
	return os.Remove(renamed)
}

// truncateSteps progressively truncates the file to hide its size.
func truncateSteps(path string, size int64) error {
	for size > 0 {
		size = size / 2
		if err := os.Truncate(path, size); err != nil {
			return fmt.Errorf("%s: %v", path, unwrapErr(err))
		}
	}
	return nil
}

// renameSteps renames file to progressively shorter names. R1.4 (remove).
func renameSteps(cfg config, path string) (string, error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	current := path
	for length := len(name); length > 0; length-- {
		newName := strings.Repeat("0", length)
		newPath := filepath.Join(dir, newName)
		if err := os.Rename(current, newPath); err != nil {
			// If rename fails, stop renaming and remove current.
			return current, nil
		}
		if cfg.verbose {
			fmt.Fprintf(os.Stderr, "%s: %s: renamed to %s\n",
				progName, current, newPath)
		}
		current = newPath
	}
	return current, nil
}

// printProgress prints verbose progress for a pass. R2.1.
// D1: format matches GNU shred: 'shred: FILE: pass N/N (kind)...'
func printProgress(path string, pass, total int, kind string) {
	fmt.Fprintf(os.Stderr, "%s: %s: pass %d/%d (%s)...\n",
		progName, path, pass, total, kind)
}

// unwrapErr extracts the inner error from *os.PathError.
func unwrapErr(err error) error {
	var pe *os.PathError
	if ok := isPathError(err, &pe); ok {
		return pe.Err
	}
	return err
}

// isPathError checks if err is *os.PathError and assigns it.
func isPathError(err error, pe **os.PathError) bool {
	target, ok := err.(*os.PathError)
	if ok {
		*pe = target
	}
	return ok
}

// parseArgs processes all command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	cfg := config{iterations: 3} // R1.1: default 3 passes
	for i := 0; i < len(args); {
		if args[i] == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			return cfg, nil
		}
		adv, err := parseArg(&cfg, args, i)
		if err != nil {
			return cfg, err
		}
		i += adv
		if cfg.showHelp || cfg.showVersion {
			return cfg, nil
		}
	}
	return cfg, nil
}

// parseArg processes one argument, returning how many args were consumed.
func parseArg(cfg *config, args []string, i int) (int, error) {
	arg := args[i]
	switch {
	case arg == "--help":
		cfg.showHelp = true
		return 1, nil
	case arg == "--version":
		cfg.showVersion = true
		return 1, nil
	case arg == "--zero":
		cfg.addZero = true
		return 1, nil
	case arg == "--verbose":
		cfg.verbose = true
		return 1, nil
	case arg == "--exact":
		cfg.exact = true
		return 1, nil
	case strings.HasPrefix(arg, "--iterations="):
		return 1, parseIterationsValue(cfg, arg[len("--iterations="):])
	case arg == "--iterations":
		return parseLongOptArg(cfg, args, i, setIterations)
	case strings.HasPrefix(arg, "--size="):
		cfg.sizeStr = arg[len("--size="):]
		return 1, nil
	case arg == "--size":
		return consumeNextArg(&cfg.sizeStr, args, i, arg)
	case strings.HasPrefix(arg, "--remove"):
		cfg.remove = true
		return 1, nil
	case strings.HasPrefix(arg, "--"):
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(cfg, args, i)
	default:
		cfg.files = append(cfg.files, arg)
		return 1, nil
	}
}

// setIterations is a setter callback for --iterations long option.
func setIterations(cfg *config, val string) error {
	return parseIterationsValue(cfg, val)
}

// parseLongOptArg consumes a long option that requires a next argument.
func parseLongOptArg(
	cfg *config, args []string, i int,
	setter func(*config, string) error,
) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf(
			"option '%s' requires an argument", args[i])
	}
	return 2, setter(cfg, args[i+1])
}

// parseIterationsValue parses and validates the iterations count. R1.2.
func parseIterationsValue(cfg *config, val string) error {
	n, err := parseInt(val)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of passes: '%s'", val)
	}
	cfg.iterations = n
	return nil
}

// parseInt parses a non-negative integer string.
func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	if len(s) == 0 {
		return 0, fmt.Errorf("empty number")
	}
	return n, nil
}

// consumeNextArg sets dst to the argument following the current one.
func consumeNextArg(
	dst *string, args []string, i int, opt string,
) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", opt)
	}
	*dst = args[i+1]
	return 2, nil
}

// parseShortFlags processes combined short flags (e.g., -vzu, -n 3).
func parseShortFlags(cfg *config, args []string, i int) (int, error) {
	flags := args[i][1:]
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'z':
			cfg.addZero = true
		case 'u':
			cfg.remove = true
		case 'v':
			cfg.verbose = true
		case 'x':
			cfg.exact = true
		case 'n':
			return consumeShortOptArgInt(
				cfg, flags[j+1:], flags[j], args, i)
		case 's':
			return consumeShortOptArgStr(
				&cfg.sizeStr, flags[j+1:], flags[j], args, i)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// consumeShortOptArgInt sets iterations from the flag tail or next arg.
func consumeShortOptArgInt(
	cfg *config, rest string, ch byte, args []string, i int,
) (int, error) {
	if rest != "" {
		return 1, parseIterationsValue(cfg, rest)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%c'", ch)
	}
	return 2, parseIterationsValue(cfg, args[i+1])
}

// consumeShortOptArgStr sets dst from the flag tail or the next argument.
func consumeShortOptArgStr(
	dst *string, rest string, ch byte, args []string, i int,
) (int, error) {
	if rest != "" {
		*dst = rest
		return 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%c'", ch)
	}
	*dst = args[i+1]
	return 2, nil
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", progName)
}

const helpText = `Usage: shred [OPTION]... FILE...
Overwrite the specified FILE(s) repeatedly, in order to make it harder
for even very expensive hardware probing to recover the data.

Mandatory arguments to long options are mandatory for short options too.
  -n, --iterations=N   overwrite N times instead of the default (3)
  -s, --size=N         shred this many bytes (suffixes like K, M, G accepted)
  -u, --remove         truncate and remove file after overwriting
  -v, --verbose        show progress
  -x, --exact          do not round file sizes up to the next full block
  -z, --zero           add a final overwrite with zeros to hide shredding
      --help           display this help and exit
      --version        output version information and exit
`

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := os.Stdout.WriteString(helpText)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(
		os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 1
	}
	return 0
}
