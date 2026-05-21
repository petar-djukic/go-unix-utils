// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func setupBinaries(t *testing.T) (string, string) {
	t.Helper()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdate")
	if err != nil {
		t.Skip("reference binary gdate not found")
	}
	return goBin, refBin
}

func TestDiffDefault(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "default_epoch0", Args: []string{"-d", "@0"}, Env: []string{"TZ=UTC"}},
		{Name: "default_epoch", Args: []string{"-d", "@1700000000"}, Env: []string{"TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFormatStrings(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "format_ymd", Args: []string{"-d", "@1700000000", "+%Y-%m-%d"}, Env: []string{"TZ=UTC"}},
		{Name: "format_hms", Args: []string{"-d", "@1700000000", "+%H:%M:%S"}, Env: []string{"TZ=UTC"}},
		{Name: "format_combined", Args: []string{"-d", "@1700000000", "+%Y/%m/%d %H:%M:%S"}, Env: []string{"TZ=UTC"}},
		{Name: "format_literal", Args: []string{"-d", "@0", "+hello"}, Env: []string{"TZ=UTC"}},
		{Name: "format_percent", Args: []string{"-d", "@0", "+100%%"}, Env: []string{"TZ=UTC"}},
		{Name: "format_empty", Args: []string{"-d", "@0", "+"}, Env: []string{"TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffSpecifiers(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "spec_Y", Args: []string{"-d", "@1700000000", "+%Y"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_m", Args: []string{"-d", "@1700000000", "+%m"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_d", Args: []string{"-d", "@1700000000", "+%d"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_H", Args: []string{"-d", "@1700000000", "+%H"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_M", Args: []string{"-d", "@1700000000", "+%M"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_S", Args: []string{"-d", "@1700000000", "+%S"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_Z", Args: []string{"-d", "@1700000000", "+%Z"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_s", Args: []string{"-d", "@1700000000", "+%s"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_N", Args: []string{"-d", "@1700000000", "+%N"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_A", Args: []string{"-d", "@1700000000", "+%A"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_B", Args: []string{"-d", "@1700000000", "+%B"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_j", Args: []string{"-d", "@1700000000", "+%j"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_u", Args: []string{"-d", "@1700000000", "+%u"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_w", Args: []string{"-d", "@1700000000", "+%w"}, Env: []string{"TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffGNUExtensions(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "spec_P_am", Args: []string{"-d", "@0", "+%P"}, Env: []string{"TZ=UTC"}},
		{Name: "spec_P_pm", Args: []string{"-d", "@43200", "+%P"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_dash_d", Args: []string{"-d", "@0", "+%-d"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_space_d", Args: []string{"-d", "@0", "+%_d"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_zero_d", Args: []string{"-d", "@0", "+%0d"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_dash_m", Args: []string{"-d", "@0", "+%-m"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_space_m", Args: []string{"-d", "@0", "+%_m"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_dash_H", Args: []string{"-d", "@0", "+%-H"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_space_H", Args: []string{"-d", "@0", "+%_H"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_dash_j", Args: []string{"-d", "@0", "+%-j"}, Env: []string{"TZ=UTC"}},
		{Name: "pad_space_j", Args: []string{"-d", "@0", "+%_j"}, Env: []string{"TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
