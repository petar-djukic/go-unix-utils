#!/usr/bin/env python3
"""Parse Claude Code stitch JSONL transcripts into tools.csv and task_turns.csv.

Walks transcript roots:
  - analysis/raw/run-43-cobbler/history/  (run 43, preserved)
  - .cobbler/history/                     (run 44, on generation branch merged to main)
  - analysis/raw/recovered/<run_id>/      (recovered Feb-Mar transcripts)

For each *-stitch-log.log, the matching *-stitch-stats.yaml in the same
directory supplies the task_id. task_id format differs across eras:
  - run-43/44 era: numeric string ("4998") joinable to tasks.csv
  - Feb-Mar era: opaque worktree slug ("generation-...-0ya"), not joinable
The `task_id_kind` column tags each row as `numeric` or `slug` so downstream
analysis can filter to the joinable subset.

Outputs:
  tools.csv       - one row per tool_use call (Read/Bash/Edit/...)
  task_turns.csv  - one row per assistant turn with token usage and
                    `tool_count_in_turn`.
"""
from __future__ import annotations

import json
import re
import sys
from collections.abc import Iterator
from pathlib import Path

import pandas as pd
import yaml

TOOLS_COLUMNS = [
    "task_id", "task_id_kind", "run_id", "session_uuid",
    "timestamp",
    "turn_index", "tool_call_index",
    "tool_name", "tool_input_summary", "bash_command_class",
    "tool_result_is_error",
    "tool_result_stdout_size", "tool_result_stderr_size",
]
TURNS_COLUMNS = [
    "task_id", "task_id_kind", "run_id", "session_uuid",
    "turn_index", "timestamp",
    "tokens_input", "tokens_output",
    "cache_creation", "cache_read",
    "model_id",
    "had_tool_use", "tool_count_in_turn",
]


_ENV_VAR_PREFIX_RE = re.compile(r"^(?:[A-Z_][A-Z0-9_]*=\S+\s+)+")
_PIPELINE_SPLIT_RE = re.compile(r"\|\||&&|;|\|")
_BINARY_RUN_RE = re.compile(
    r"^(?:\./)?(?:bin/[A-Za-z0-9_-]+|cmd/[A-Za-z0-9_-]+|go\s+run)\b"
)
_INSPECT_RE = re.compile(r"^(?:xxd|od|hexdump|wc|head|tail|diff)\b")
_SETUP_RE = re.compile(r"^(?:printf|echo|cat\s*<<)\b")


def normalize_command(cmd: str) -> str:
    """Strip leading `# comment` lines and `VAR=val ` env prefixes."""
    lines = cmd.split("\n")
    while lines and lines[0].lstrip().startswith("#"):
        lines.pop(0)
    cmd = "\n".join(lines).strip()
    return _ENV_VAR_PREFIX_RE.sub("", cmd)


def pipeline_segments(cmd: str) -> list[str]:
    """Split on |, ||, &&, ; — naive (no quote awareness, but rare in practice)."""
    return [s.strip() for s in _PIPELINE_SPLIT_RE.split(cmd) if s.strip()]


def _classify_segment(seg: str) -> str | None:
    """Classify a single pipeline segment by its first verb. None = unknown."""
    if re.match(r"go\s+(?:build|install)\b", seg):
        return "go_build"
    if re.match(r"go\s+vet\b", seg):
        return "go_vet"
    if re.match(r"go\s+test\b", seg):
        return "go_test"
    if re.match(r"go\s+mod\b", seg):
        return "go_mod"
    if _BINARY_RUN_RE.match(seg):
        return "binary_run"
    if re.match(r"golangci-lint\b", seg):
        return "lint"
    if re.match(r"gofmt\b", seg):
        return "format"
    if re.match(r"mage\b", seg):
        return "mage"
    git_m = re.match(r"git\s+(\w+)", seg)
    if git_m:
        return f"git_{git_m.group(1)}"
    if re.match(r"which\b", seg):
        return "env_check"
    if re.match(r"(?:pwd|cd)\b", seg):
        return "env_inspect"
    if re.match(r"go\s+\w+", seg):
        return "go_other"
    for verb in ("mkdir", "rm", "cp", "mv", "find", "grep", "cat", "head",
                 "tail", "wc", "echo", "ls", "diff", "xxd", "od"):
        if re.match(rf"{verb}\b", seg):
            return verb
    return None


def classify_bash(command: str) -> str:
    """Walk the pipeline; if leading segment is setup (printf/echo/cat <<)
    and a downstream segment runs a compiled binary or inspects its output,
    label the whole call as `binary_exercise`. Otherwise classify by the
    first meaningful verb."""
    cmd = normalize_command(command)
    if not cmd:
        return "empty"

    segs = pipeline_segments(cmd)
    if not segs:
        return "other"
    first = segs[0]

    # printf/echo/cat-here-doc piped into a binary or inspector → exercise
    if _SETUP_RE.match(first):
        for seg in segs[1:]:
            if _BINARY_RUN_RE.match(seg) or _INSPECT_RE.match(seg):
                return "binary_exercise"
        return "setup"

    label = _classify_segment(first)
    return label if label is not None else "other"


def summarize_input(tool_name: str, tool_input: dict) -> str:
    raw: str
    if tool_name == "Bash":
        raw = str(tool_input.get("command", ""))
    elif tool_name in ("Read", "Edit", "Write", "NotebookEdit"):
        raw = str(tool_input.get("file_path", ""))
    elif tool_name in ("Grep", "Glob"):
        raw = str(tool_input.get("pattern", ""))
    elif tool_name == "Task":
        raw = str(tool_input.get("description", ""))
    else:
        raw = json.dumps(tool_input, default=str)
    return raw[:200]


def parse_jsonl(path: Path) -> Iterator[dict]:
    with path.open() as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                yield json.loads(line)
            except json.JSONDecodeError:
                continue


def load_stats(stats_path: Path) -> dict | None:
    if not stats_path.exists():
        return None
    try:
        with stats_path.open() as f:
            return yaml.safe_load(f)
    except Exception:
        return None


def task_id_kind(task_id: str) -> str:
    return "numeric" if task_id.isdigit() else "slug"


def process_log(log_path: Path, run_id: str) -> tuple[list[dict], list[dict]]:
    prefix = log_path.name[:19]
    stats_path = log_path.parent / f"{prefix}-stitch-stats.yaml"
    stats = load_stats(stats_path)
    if stats is None or "task_id" not in stats:
        return [], []
    task_id = str(stats["task_id"])
    kind = task_id_kind(task_id)

    session_uuid: str | None = None
    pending: dict[str, dict] = {}
    turn_index = 0
    tool_call_index = 0
    tools_rows: list[dict] = []
    turns_rows: list[dict] = []

    for obj in parse_jsonl(log_path):
        t = obj.get("type")
        if t == "system" and obj.get("subtype") == "init":
            if session_uuid is None:
                session_uuid = obj.get("session_id")
        elif t == "assistant":
            msg = obj.get("message", {})
            usage = msg.get("usage", {})
            content = msg.get("content", [])
            tool_uses = [c for c in content if c.get("type") == "tool_use"]
            turn_index += 1
            turns_rows.append({
                "task_id": task_id,
                "task_id_kind": kind,
                "run_id": run_id,
                "session_uuid": session_uuid,
                "turn_index": turn_index,
                "timestamp": obj.get("timestamp"),
                "tokens_input": int(usage.get("input_tokens", 0) or 0),
                "tokens_output": int(usage.get("output_tokens", 0) or 0),
                "cache_creation": int(usage.get("cache_creation_input_tokens", 0) or 0),
                "cache_read": int(usage.get("cache_read_input_tokens", 0) or 0),
                "model_id": msg.get("model"),
                "had_tool_use": bool(tool_uses),
                "tool_count_in_turn": len(tool_uses),
            })
            for tu in tool_uses:
                tool_call_index += 1
                tu_id = tu.get("id")
                tool_name = tu.get("name", "")
                input_dict = tu.get("input", {})
                summary = summarize_input(tool_name, input_dict)
                bash_class = (
                    classify_bash(str(input_dict.get("command", "")))
                    if tool_name == "Bash"
                    else None
                )
                pending[tu_id] = {
                    "turn_index": turn_index,
                    "tool_call_index": tool_call_index,
                    "tool_name": tool_name,
                    "input_summary": summary,
                    "bash_class": bash_class,
                    "timestamp": obj.get("timestamp"),
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
                is_error = bool(c.get("is_error", False))
                tur = obj.get("tool_use_result", {}) or {}
                if p["tool_name"] == "Bash" and isinstance(tur, dict):
                    stdout_size = len(str(tur.get("stdout", "")))
                    stderr_size = len(str(tur.get("stderr", "")))
                else:
                    content_field = c.get("content", "")
                    stdout_size = len(content_field) if isinstance(content_field, str) else len(json.dumps(content_field, default=str))
                    stderr_size = 0
                tools_rows.append({
                    "task_id": task_id,
                    "task_id_kind": kind,
                    "run_id": run_id,
                    "session_uuid": session_uuid,
                    "timestamp": p["timestamp"] or obj.get("timestamp"),
                    "turn_index": p["turn_index"],
                    "tool_call_index": p["tool_call_index"],
                    "tool_name": p["tool_name"],
                    "tool_input_summary": p["input_summary"],
                    "bash_command_class": p["bash_class"],
                    "tool_result_is_error": is_error,
                    "tool_result_stdout_size": stdout_size,
                    "tool_result_stderr_size": stderr_size,
                })

    # tool_uses with no matching tool_result (interrupted sessions)
    for tu_id, p in pending.items():
        tools_rows.append({
            "task_id": task_id,
            "task_id_kind": kind,
            "run_id": run_id,
            "session_uuid": session_uuid,
            "timestamp": p["timestamp"],
            "turn_index": p["turn_index"],
            "tool_call_index": p["tool_call_index"],
            "tool_name": p["tool_name"],
            "tool_input_summary": p["input_summary"],
            "bash_command_class": p["bash_class"],
            "tool_result_is_error": None,
            "tool_result_stdout_size": None,
            "tool_result_stderr_size": None,
        })

    return tools_rows, turns_rows


def find_logs(repo_root: Path) -> Iterator[tuple[str, Path]]:
    recovered = repo_root / "analysis" / "raw" / "recovered"
    if recovered.exists():
        for d in sorted(recovered.iterdir()):
            if not d.is_dir():
                continue
            for f in sorted(d.glob("*-stitch-log.log")):
                yield d.name, f
    run43_root = repo_root / "analysis" / "raw" / "run-43-cobbler" / "history"
    if run43_root.exists():
        for f in sorted(run43_root.glob("*-stitch-log.log")):
            yield "gh-4994-run43", f
    run44_root = repo_root / ".cobbler" / "history"
    if run44_root.exists():
        for f in sorted(run44_root.glob("*-stitch-log.log")):
            yield "run44", f


def _self_check_classifier() -> None:
    """Sanity asserts so refactor regressions show up loudly."""
    cases = [
        ("go build ./pkg/testutils/...", "go_build"),
        ("go vet ./cmd/ts/...", "go_vet"),
        ("go test ./cmd/cat/... -v", "go_test"),
        ("printf 'a\\nb\\n' | go run ./cmd/cat", "binary_exercise"),
        ("printf 'x' | ./bin/wc -l", "binary_exercise"),
        ("printf 'x' | xxd", "binary_exercise"),
        ("printf 'x'", "setup"),
        ("# Test R3.2\nprintf 'a\\n' | ./bin/cat", "binary_exercise"),
        ("CGO_ENABLED=0 go build ./cmd/cat", "go_build"),
        ("./bin/du -sh ./pkg/", "binary_run"),
        ("golangci-lint run ./pkg/...", "lint"),
        ("gofmt -l ./", "format"),
        ("which golangci-lint", "env_check"),
        ("pwd", "env_inspect"),
        ("git status", "git_status"),
        ("mage stats:loc", "mage"),
        ("mkdir -p /tmp/foo", "mkdir"),
        ("ls -la", "ls"),
    ]
    failures = []
    for cmd, expected in cases:
        got = classify_bash(cmd)
        if got != expected:
            failures.append((cmd, expected, got))
    if failures:
        for cmd, exp, got in failures:
            print(f"  CLASSIFIER FAIL: {cmd!r}  expected={exp}  got={got}", file=sys.stderr)
        sys.exit(2)


def main() -> int:
    _self_check_classifier()
    repo_root = Path(__file__).resolve().parent.parent.parent
    all_tools: list[dict] = []
    all_turns: list[dict] = []
    n_logs = 0
    n_skipped = 0

    for run_id, log_path in find_logs(repo_root):
        tools, turns = process_log(log_path, run_id)
        if not tools and not turns:
            n_skipped += 1
            continue
        all_tools.extend(tools)
        all_turns.extend(turns)
        n_logs += 1

    print(f"Processed {n_logs} stitch JSONL transcripts ({n_skipped} skipped — no stats.yaml or empty)")

    tools_df = pd.DataFrame(all_tools, columns=TOOLS_COLUMNS)
    turns_df = pd.DataFrame(all_turns, columns=TURNS_COLUMNS)

    out_dir = repo_root / "analysis" / "datasets"
    tools_df.to_csv(out_dir / "tools.csv", index=False)
    turns_df.to_csv(out_dir / "task_turns.csv", index=False)

    print(f"\ntools.csv: {len(tools_df)} rows")
    print(f"  tool_name top: {tools_df['tool_name'].value_counts().head(10).to_dict()}")
    print(f"  bash_command_class top: {tools_df['bash_command_class'].value_counts().head(10).to_dict()}")
    print(f"  is_error breakdown: {tools_df['tool_result_is_error'].value_counts(dropna=False).to_dict()}")

    print(f"\ntask_turns.csv: {len(turns_df)} rows")
    print(f"  unique tasks: {turns_df['task_id'].nunique()}")
    print(f"  task_id_kind: {turns_df['task_id_kind'].value_counts().to_dict()}")
    print(f"  total tokens_input: {turns_df['tokens_input'].sum():,}")
    print(f"  total tokens_output: {turns_df['tokens_output'].sum():,}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
