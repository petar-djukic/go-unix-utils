#!/usr/bin/env python3
"""Per-Bash-call success/failure flags for build/test/lint phases.

`tools.csv` records `tool_result_is_error`, but Claude Code's Bash tool sets
that flag only on tool-level errors (timeout, interrupted). A `go build` that
fails with compile errors comes back with `is_error=False`. The actual
failure signal lives in stdout/stderr content tokens.

This script walks the same JSONL logs as `extract_transcripts.py` and writes
`bash_outcomes.csv`. Each row is a single Bash tool call mapped to one of
{build, test, lint} via `bash_command_class`, with `failed=True` when its
output matches a phase-specific failure pattern.

Failure patterns (case-sensitive on stdout+stderr concatenated):

  build (go_build)
    \\.go:\\d+:\\d+: ; cannot find package ; undefined: ;
    imported and not used ; syntax error ; build failed ; no Go files

  test (go_test)
    FAIL\\t ; FAIL: ; --- FAIL ; build failed ; \\.go:\\d+:\\d+:

  lint (go_vet, lint)
    \\.go:\\d+:\\d+: ; level=error ; issues found ; vet: ;
    non-empty stderr after stripping shell-cwd-reset noise

`interrupted=True` from the tool_use_result also counts as failure.
"""
from __future__ import annotations

import json
import re
import sys
from collections.abc import Iterator
from pathlib import Path

import pandas as pd

sys.path.insert(0, str(Path(__file__).parent))
from extract_transcripts import (  # noqa: E402
    classify_bash,
    find_logs,
    load_stats,
    parse_jsonl,
    task_id_kind,
)

OUTCOME_COLUMNS = [
    "task_id", "task_id_kind", "run_id", "session_uuid",
    "turn_index", "tool_call_index",
    "bash_command_class", "phase",
    "failed", "interrupted",
    "stdout_size", "stderr_size",
]

CLASS_TO_PHASE = {
    "go_build": "build",
    "go_test": "test",
    "go_vet": "lint",
    "lint": "lint",
}

_GO_LINE_CITE = re.compile(r"\.go:\d+:\d+:")
_BUILD_PATTERNS = [
    _GO_LINE_CITE,
    re.compile(r"cannot find package"),
    re.compile(r"undefined: "),
    re.compile(r"imported and not used"),
    re.compile(r"syntax error"),
    re.compile(r"build failed"),
    re.compile(r"no Go files"),
]
_TEST_PATTERNS = [
    re.compile(r"FAIL\t"),
    re.compile(r"FAIL: "),
    re.compile(r"--- FAIL"),
    re.compile(r"build failed"),
    _GO_LINE_CITE,
]
_LINT_PATTERNS = [
    _GO_LINE_CITE,
    re.compile(r"level=error"),
    re.compile(r"issues found"),
    re.compile(r"vet: "),
]
_SHELL_CWD_NOISE = re.compile(r"^Shell cwd was reset to .*$", re.MULTILINE)


def detect_failure(phase: str, stdout: str, stderr: str) -> bool:
    combined = stdout + "\n" + stderr
    if phase == "build":
        return any(p.search(combined) for p in _BUILD_PATTERNS)
    if phase == "test":
        return any(p.search(combined) for p in _TEST_PATTERNS)
    if phase == "lint":
        if any(p.search(combined) for p in _LINT_PATTERNS):
            return True
        cleaned = _SHELL_CWD_NOISE.sub("", stderr).strip()
        return bool(cleaned)
    return False


def process_log(log_path: Path, run_id: str) -> list[dict]:
    prefix = log_path.name[:19]
    stats_path = log_path.parent / f"{prefix}-stitch-stats.yaml"
    stats = load_stats(stats_path)
    if stats is None or "task_id" not in stats:
        return []
    task_id = str(stats["task_id"])
    kind = task_id_kind(task_id)

    session_uuid: str | None = None
    pending: dict[str, dict] = {}
    turn_index = 0
    tool_call_index = 0
    rows: list[dict] = []

    for obj in parse_jsonl(log_path):
        t = obj.get("type")
        if t == "system" and obj.get("subtype") == "init":
            if session_uuid is None:
                session_uuid = obj.get("session_id")
        elif t == "assistant":
            msg = obj.get("message", {})
            content = msg.get("content", [])
            tool_uses = [c for c in content if c.get("type") == "tool_use"]
            turn_index += 1
            for tu in tool_uses:
                tool_call_index += 1
                if tu.get("name") != "Bash":
                    continue
                cmd = (tu.get("input") or {}).get("command", "")
                bash_class = classify_bash(str(cmd))
                phase = CLASS_TO_PHASE.get(bash_class)
                if phase is None:
                    continue
                pending[tu.get("id")] = {
                    "turn_index": turn_index,
                    "tool_call_index": tool_call_index,
                    "bash_class": bash_class,
                    "phase": phase,
                }
        elif t == "user":
            msg = obj.get("message", {})
            for c in msg.get("content", []):
                if c.get("type") != "tool_result":
                    continue
                tu_id = c.get("tool_use_id")
                p = pending.pop(tu_id, None)
                if p is None:
                    continue
                tur = obj.get("tool_use_result", {}) or {}
                if not isinstance(tur, dict):
                    tur = {}
                stdout = str(tur.get("stdout", ""))
                stderr = str(tur.get("stderr", ""))
                interrupted = bool(tur.get("interrupted", False))
                failed = interrupted or detect_failure(p["phase"], stdout, stderr)
                rows.append({
                    "task_id": task_id,
                    "task_id_kind": kind,
                    "run_id": run_id,
                    "session_uuid": session_uuid,
                    "turn_index": p["turn_index"],
                    "tool_call_index": p["tool_call_index"],
                    "bash_command_class": p["bash_class"],
                    "phase": p["phase"],
                    "failed": failed,
                    "interrupted": interrupted,
                    "stdout_size": len(stdout),
                    "stderr_size": len(stderr),
                })

    for tu_id, p in pending.items():
        rows.append({
            "task_id": task_id,
            "task_id_kind": kind,
            "run_id": run_id,
            "session_uuid": session_uuid,
            "turn_index": p["turn_index"],
            "tool_call_index": p["tool_call_index"],
            "bash_command_class": p["bash_class"],
            "phase": p["phase"],
            "failed": True,
            "interrupted": True,
            "stdout_size": 0,
            "stderr_size": 0,
        })
    return rows


def main(repo_root: Path) -> int:
    out = repo_root / "analysis" / "datasets" / "bash_outcomes.csv"
    all_rows: list[dict] = []
    log_count = 0
    for run_id, log_path in find_logs(repo_root):
        all_rows.extend(process_log(log_path, run_id))
        log_count += 1
    df = pd.DataFrame(all_rows, columns=OUTCOME_COLUMNS)
    out.parent.mkdir(parents=True, exist_ok=True)
    df.to_csv(out, index=False)
    print(f"logs scanned: {log_count}")
    print(f"rows: {len(df)}")
    print(df.groupby(["phase", "failed"]).size().unstack(fill_value=0))
    return 0


if __name__ == "__main__":
    repo = Path(__file__).resolve().parents[2]
    sys.exit(main(repo))
