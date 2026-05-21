package main

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	inputRange [2]int
	hasRange   bool
	headCount  int
	hasHead    bool
	repeat     bool
	outputFile string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
		os.Exit(1)
	}

	if opts.hasRange && len(files) > 0 {
		fmt.Fprintf(os.Stderr, "shuf: extra operand %q\n", files[0])
		os.Exit(1)
	}

	lines, err := collectLines(opts, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
		os.Exit(1)
	}

	w, closer, err := openOutput(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
		os.Exit(1)
	}

	if err := writeOutput(w, lines, opts); err != nil {
		fmt.Fprintf(os.Stderr, "shuf: write error: %v\n", err)
		os.Exit(1)
	}

	if closer != nil {
		if err := closer.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "shuf: write error: %v\n", err)
			os.Exit(1)
		}
	}
}

func collectLines(opts options, files []string) ([]string, error) {
	if opts.hasRange {
		lo, hi := opts.inputRange[0], opts.inputRange[1]
		lines := make([]string, 0, hi-lo+1)
		for i := lo; i <= hi; i++ {
			lines = append(lines, strconv.Itoa(i))
		}
		return lines, nil
	}
	return readLines(files)
}

func openOutput(opts options) (*bufio.Writer, *os.File, error) {
	if opts.outputFile == "" {
		return bufio.NewWriter(os.Stdout), nil, nil
	}
	f, err := os.Create(opts.outputFile)
	if err != nil {
		return nil, nil, err
	}
	return bufio.NewWriter(f), f, nil
}

func writeOutput(w *bufio.Writer, lines []string, opts options) error {
	if len(lines) == 0 {
		return w.Flush()
	}

	if opts.repeat {
		return writeRepeat(w, lines, opts)
	}

	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})

	n := len(lines)
	if opts.hasHead && opts.headCount < n {
		n = opts.headCount
	}

	for i := range n {
		if _, err := fmt.Fprintln(w, lines[i]); err != nil {
			return err
		}
	}
	return w.Flush()
}

func writeRepeat(w *bufio.Writer, lines []string, opts options) error {
	if opts.hasHead {
		for range opts.headCount {
			if _, err := fmt.Fprintln(w, lines[rand.IntN(len(lines))]); err != nil {
				return err
			}
		}
		return w.Flush()
	}
	for {
		if _, err := fmt.Fprintln(w, lines[rand.IntN(len(lines))]); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if arg != "-" && strings.HasPrefix(arg, "-") {
			n, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		files = append(files, arg)
	}
	return opts, files, nil
}

func parseLongFlag(flag string, rest []string, opts *options) (int, error) {
	switch {
	case flag == "--input-range":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '--input-range' requires an argument")
		}
		return 1, parseRange(rest[0], opts)
	case strings.HasPrefix(flag, "--input-range="):
		return 0, parseRange(flag[len("--input-range="):], opts)
	case flag == "--head-count":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '--head-count' requires an argument")
		}
		return 1, parseHeadCount(rest[0], opts)
	case strings.HasPrefix(flag, "--head-count="):
		return 0, parseHeadCount(flag[len("--head-count="):], opts)
	case flag == "--repeat":
		opts.repeat = true
		return 0, nil
	case flag == "--output":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '--output' requires an argument")
		}
		opts.outputFile = rest[0]
		return 1, nil
	case strings.HasPrefix(flag, "--output="):
		opts.outputFile = flag[len("--output="):]
		return 0, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, rest []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'i':
			val, n, err := shortFlagValue(flags[j+1:], rest, consumed, "-i")
			if err != nil {
				return 0, err
			}
			consumed += n
			return consumed, parseRange(val, opts)
		case 'n':
			val, n, err := shortFlagValue(flags[j+1:], rest, consumed, "-n")
			if err != nil {
				return 0, err
			}
			consumed += n
			return consumed, parseHeadCount(val, opts)
		case 'o':
			val, n, err := shortFlagValue(flags[j+1:], rest, consumed, "-o")
			if err != nil {
				return 0, err
			}
			consumed += n
			opts.outputFile = val
			return consumed, nil
		case 'r':
			opts.repeat = true
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func shortFlagValue(tail string, rest []string, consumed int, name string) (string, int, error) {
	if tail != "" {
		return tail, 0, nil
	}
	if consumed < len(rest) {
		return rest[consumed], 1, nil
	}
	return "", 0, fmt.Errorf("option '%s' requires an argument", name)
}

func parseRange(s string, opts *options) error {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid input range: %q", s)
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid input range: %q", s)
	}
	hi, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid input range: %q", s)
	}
	if lo > hi {
		return fmt.Errorf("invalid input range: %q", s)
	}
	opts.inputRange = [2]int{lo, hi}
	opts.hasRange = true
	return nil
}

func parseHeadCount(s string, opts *options) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid line count: %q", s)
	}
	opts.headCount = n
	opts.hasHead = true
	return nil
}

func readLines(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"-"}
	}

	var lines []string
	for _, name := range args {
		ls, err := readFileLines(name)
		if err != nil {
			return nil, err
		}
		lines = append(lines, ls...)
	}
	return lines, nil
}

func readFileLines(name string) ([]string, error) {
	var f *os.File
	if name == "-" {
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
