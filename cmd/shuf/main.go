package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	inputRange   [2]int
	hasRange     bool
	headCount    int
	hasHead      bool
	repeat       bool
	outputFile   string
	randomSource string
	zeroTerm     bool
	echo         bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
		os.Exit(1)
	}

	if opts.hasRange && (opts.echo || len(files) > 0) {
		if opts.echo {
			fmt.Fprintf(os.Stderr, "shuf: cannot combine -e and -i options\n")
		} else {
			fmt.Fprintf(os.Stderr, "shuf: extra operand %q\n", files[0])
		}
		os.Exit(1)
	}

	rng, err := makeRNG(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
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

	if err := writeOutput(w, lines, opts, rng); err != nil {
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

func makeRNG(opts options) (*rand.Rand, error) {
	if opts.randomSource == "" {
		return nil, nil
	}
	data, err := os.ReadFile(opts.randomSource)
	if err != nil {
		return nil, err
	}
	var seed [32]byte
	copy(seed[:], data)
	src := rand.NewChaCha8(seed)
	return rand.New(src), nil
}

func randShuffle(rng *rand.Rand, n int, swap func(i, j int)) {
	if rng != nil {
		rng.Shuffle(n, swap)
	} else {
		rand.Shuffle(n, swap)
	}
}

func randIntN(rng *rand.Rand, n int) int {
	if rng != nil {
		return rng.IntN(n)
	}
	return rand.IntN(n)
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
	if opts.echo {
		return files, nil
	}
	return readLines(files, opts.zeroTerm)
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

func writeLine(w *bufio.Writer, line string, zeroTerm bool) error {
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	if zeroTerm {
		return w.WriteByte(0)
	}
	return w.WriteByte('\n')
}

func writeOutput(w *bufio.Writer, lines []string, opts options, rng *rand.Rand) error {
	if len(lines) == 0 {
		return w.Flush()
	}

	if opts.repeat {
		return writeRepeat(w, lines, opts, rng)
	}

	randShuffle(rng, len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})

	n := len(lines)
	if opts.hasHead && opts.headCount < n {
		n = opts.headCount
	}

	for i := range n {
		if err := writeLine(w, lines[i], opts.zeroTerm); err != nil {
			return err
		}
	}
	return w.Flush()
}

func writeRepeat(w *bufio.Writer, lines []string, opts options, rng *rand.Rand) error {
	if opts.hasHead {
		for range opts.headCount {
			if err := writeLine(w, lines[randIntN(rng, len(lines))], opts.zeroTerm); err != nil {
				return err
			}
		}
		return w.Flush()
	}
	for {
		if err := writeLine(w, lines[randIntN(rng, len(lines))], opts.zeroTerm); err != nil {
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
	case flag == "--random-source":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '--random-source' requires an argument")
		}
		opts.randomSource = rest[0]
		return 1, nil
	case strings.HasPrefix(flag, "--random-source="):
		opts.randomSource = flag[len("--random-source="):]
		return 0, nil
	case flag == "--zero-terminated":
		opts.zeroTerm = true
		return 0, nil
	case flag == "--echo":
		opts.echo = true
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
		case 'z':
			opts.zeroTerm = true
		case 'e':
			opts.echo = true
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
	lo, hi, ok := splitRange(s)
	if !ok {
		return fmt.Errorf("invalid input range: %q", s)
	}
	if lo > hi {
		return fmt.Errorf("invalid input range: %q", s)
	}
	opts.inputRange = [2]int{lo, hi}
	opts.hasRange = true
	return nil
}

func splitRange(s string) (int, int, bool) {
	idx := strings.Index(s, "-")
	if idx < 0 {
		return 0, 0, false
	}
	if idx == 0 {
		idx = strings.Index(s[1:], "-")
		if idx < 0 {
			return 0, 0, false
		}
		idx++
	}
	lo, err := strconv.Atoi(s[:idx])
	if err != nil {
		return 0, 0, false
	}
	hi, err := strconv.Atoi(s[idx+1:])
	if err != nil {
		return 0, 0, false
	}
	return lo, hi, true
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

func readLines(args []string, zeroTerm bool) ([]string, error) {
	if len(args) == 0 {
		args = []string{"-"}
	}

	var lines []string
	for _, name := range args {
		ls, err := readFileLines(name, zeroTerm)
		if err != nil {
			return nil, err
		}
		lines = append(lines, ls...)
	}
	return lines, nil
}

func readFileLines(name string, zeroTerm bool) ([]string, error) {
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

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	var delim byte = '\n'
	if zeroTerm {
		delim = 0
	}

	// Remove trailing delimiter if present
	data = bytes.TrimSuffix(data, []byte{delim})
	if len(data) == 0 {
		return []string{""}, nil
	}

	parts := bytes.Split(data, []byte{delim})
	lines := make([]string, len(parts))
	for i, p := range parts {
		lines[i] = string(p)
	}
	return lines, nil
}

