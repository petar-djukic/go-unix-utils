// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sponge implements srd007-sponge: soak all of stdin before writing
// to a named output file, preventing read-write conflicts in pipelines.
package main

import (
	"flag"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var appendMode = flag.Bool("a", false, "append to output file")

func main() {
	sys.InstallSIGPIPEHandler()
	flag.Parse()
	os.Exit(run())
}

func run() int {
	return 0
}

func readAllStdin() ([]byte, string, error) {
	return nil, "", nil
}

func writeOutputFile(filename string, data []byte, tmpFile string) error {
	return nil
}

func prependOriginalContent(filename string, data []byte) ([]byte, error) {
	return nil, nil
}

func writeToStdout(data []byte, tmpFile string) error {
	return nil
}

func cleanupTempFile(path string) {
}
