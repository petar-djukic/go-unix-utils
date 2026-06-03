// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/blake2b"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var crcTable [256]uint32

func init() {
	const poly = 0x04C11DB7
	for i := range 256 {
		c := uint32(i) << 24
		for range 8 {
			if c&0x80000000 != 0 {
				c = (c << 1) ^ poly
			} else {
				c <<= 1
			}
		}
		crcTable[i] = c
	}
}

type options struct {
	algorithm string
	untagged  bool
	raw       bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files := parseArgs(os.Args[1:])
	alg := opts.algorithm
	if alg == "" || alg == "crc" {
		runCRC(files)
		return
	}
	cfg, err := algorithmConfig(alg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
		os.Exit(1)
	}
	if opts.raw {
		if len(files) > 1 {
			fmt.Fprintf(os.Stderr, "cksum: the --raw option is not supported with multiple files\n")
			os.Exit(1)
		}
		os.Exit(digestRaw(files, cfg))
	}
	os.Exit(hashutil.DigestFiles(files, cfg, false, !opts.untagged, os.Stdout, os.Stderr))
}

func runCRC(files []string) {
	exitCode := 0
	if len(files) == 0 {
		if err := crcStdin(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
			exitCode = 1
		}
	} else {
		for _, file := range files {
			if err := crcFile(file, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
				exitCode = 1
			}
		}
	}
	os.Exit(exitCode)
}

func algorithmConfig(alg string) (hashutil.HashConfig, error) {
	switch alg {
	case "sha1":
		return hashutil.HashConfig{Algorithm: "SHA1", NewHash: sha1.New, DigestLen: 20}, nil
	case "sha224":
		return hashutil.HashConfig{Algorithm: "SHA224", NewHash: sha256.New224, DigestLen: 28}, nil
	case "sha256":
		return hashutil.HashConfig{Algorithm: "SHA256", NewHash: sha256.New, DigestLen: 32}, nil
	case "sha384":
		return hashutil.HashConfig{Algorithm: "SHA384", NewHash: sha512.New384, DigestLen: 48}, nil
	case "sha512":
		return hashutil.HashConfig{Algorithm: "SHA512", NewHash: sha512.New, DigestLen: 64}, nil
	case "blake2b":
		return hashutil.HashConfig{
			Algorithm: "BLAKE2b",
			NewHash:   func() hash.Hash { h, _ := blake2b.New512(nil); return h },
			DigestLen: 64,
		}, nil
	case "sm3":
		return hashutil.HashConfig{Algorithm: "SM3", NewHash: newSM3, DigestLen: 32}, nil
	default:
		return hashutil.HashConfig{}, fmt.Errorf("unrecognized algorithm: %s", alg)
	}
}

func digestRaw(files []string, cfg hashutil.HashConfig) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	for _, file := range files {
		r, err := openInput(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
			return 1
		}
		h := cfg.NewHash()
		_, err = io.Copy(h, r)
		r.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cksum: %v\n", err)
			return 1
		}
		os.Stdout.Write(h.Sum(nil))
	}
	return 0
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			i += parseLongFlag(args[i:], &opts)
			continue
		}
		if arg != "-" && strings.HasPrefix(arg, "-") {
			i += parseShortFlag(args[i:], &opts)
			continue
		}
		files = append(files, arg)
		i++
	}
	return opts, files
}

func parseLongFlag(remaining []string, opts *options) int {
	arg := remaining[0]
	switch {
	case arg == "--untagged":
		opts.untagged = true
	case arg == "--raw":
		opts.raw = true
	case arg == "--algorithm" && len(remaining) >= 2:
		opts.algorithm = remaining[1]
		return 2
	case strings.HasPrefix(arg, "--algorithm="):
		opts.algorithm = arg[len("--algorithm="):]
	}
	return 1
}

func parseShortFlag(remaining []string, opts *options) int {
	arg := remaining[0]
	if len(arg) >= 2 && arg[1] == 'a' {
		if len(arg) > 2 {
			opts.algorithm = arg[2:]
			return 1
		}
		if len(remaining) >= 2 {
			opts.algorithm = remaining[1]
			return 2
		}
	}
	return 1
}

func crcStdin(stdout io.Writer) error {
	crc, size, err := computeCRC(os.Stdin)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d %d\n", crc, size)
	return nil
}

func crcFile(file string, stdout io.Writer) error {
	r, err := openInput(file)
	if err != nil {
		return err
	}
	defer r.Close()
	crc, size, err := computeCRC(r)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%d %d %s\n", crc, size, file)
	return nil
}

func computeCRC(r io.Reader) (uint32, int64, error) {
	var crc uint32
	var size int64
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(b)]
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	for n := size; n > 0; n >>= 8 {
		crc = (crc << 8) ^ crcTable[(crc>>24)^uint32(n&0xFF)]
	}
	crc = ^crc
	return crc, size, nil
}

func openInput(file string) (io.ReadCloser, error) {
	if file == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(file)
}
