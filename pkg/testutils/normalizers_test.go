// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"testing"
)

func TestTimestampNormalizer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ISO 8601 datetime",
			input: "event at 2026-03-28T12:34:56 done",
			want:  "event at <TIMESTAMP> done",
		},
		{
			name:  "ISO 8601 with space separator",
			input: "event at 2026-03-28 12:34:56 done",
			want:  "event at <TIMESTAMP> done",
		},
		{
			name:  "ISO 8601 with fractional seconds",
			input: "event at 2026-03-28T12:34:56.789 done",
			want:  "event at <TIMESTAMP> done",
		},
		{
			name:  "ISO 8601 with timezone offset",
			input: "event at 2026-03-28T12:34:56+05:30 done",
			want:  "event at <TIMESTAMP> done",
		},
		{
			name:  "ISO 8601 with Z timezone",
			input: "event at 2026-03-28T12:34:56Z done",
			want:  "event at <TIMESTAMP> done",
		},
		{
			name:  "Unix epoch seconds",
			input: "timestamp 1711612496 end",
			want:  "timestamp <TIMESTAMP> end",
		},
		{
			name:  "full coreutils date format",
			input: "Mon Jan  2 15:04:05 2006",
			want:  "<TIMESTAMP>",
		},
		{
			name:  "month day time with seconds",
			input: "logged Jan  2 15:04:05 event",
			want:  "logged <TIMESTAMP> event",
		},
		{
			name:  "month day time without seconds",
			input: "file modified Mar 15 09:42 name.txt",
			want:  "file modified <TIMESTAMP> name.txt",
		},
		{
			name:  "time only HH:MM:SS",
			input: "at 12:34:56 done",
			want:  "at <TIMESTAMP> done",
		},
		{
			name:  "time only with fractional",
			input: "at 12:34:56.123456 done",
			want:  "at <TIMESTAMP> done",
		},
		{
			name:  "no timestamps unchanged",
			input: "hello world\nno timestamps here\n",
			want:  "hello world\nno timestamps here\n",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "multiple timestamps",
			input: "start 2026-01-01T00:00:00 end 2026-12-31T23:59:59",
			want:  "start <TIMESTAMP> end <TIMESTAMP>",
		},
		{
			name:  "non-timestamp digits unchanged",
			input: "port 8080 and pid 12345",
			want:  "port 8080 and pid 12345",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TimestampNormalizer([]byte(tc.input))
			if string(got) != tc.want {
				t.Errorf("TimestampNormalizer(%q)\n  got:  %q\n  want: %q",
					tc.input, string(got), tc.want)
			}
		})
	}
}
