#!/usr/bin/env python3
"""Recover .cobbler/history/* files from git into analysis/raw/recovered/.

Some early generation runs accidentally committed transcript data despite the
.gitignore rule. 178 stitch-log.log JSONL transcripts (>1KB) plus ~1,400
supporting yaml files exist in git for runs from 2026-02-25 to 2026-03-06.

Recovery scope (issue #5020):
  - *-stitch-log.log, *-measure-log.log    (JSONL session transcripts)
  - *-stitch-stats.yaml, *-measure-stats.yaml  (per-task structured summary)
  - *-stitch-report.yaml                    (per-task outcome)
  - *-stitch-prompt.yaml, *-measure-prompt.yaml  (prompt content for context)
Skipped:
  - *-orchestrator.log (out of scope)
  - anything <= 100 bytes (placeholder from gitignore-bypass commits)

Each file is bucketed by run_id by reading the matching *-stats.yaml's
`started_at` UTC timestamp and looking it up against runs.csv windows.
Files whose prefix can't be resolved fall back to America/New_York TZ
inference from the filename, then to a `recovered/orphan/` bucket if no run
window matches.
"""
from __future__ import annotations

import re
import subprocess
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path

import pandas as pd
import yaml

PATH_PATTERN = re.compile(
    r"^\.cobbler/history/(?P<prefix>\d{4}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2})-(?P<rest>.+)$"
)
RECOVER_SUFFIXES = (
    "stitch-log.log",
    "stitch-stats.yaml",
    "stitch-report.yaml",
    "stitch-prompt.yaml",
    "measure-log.log",
    "measure-stats.yaml",
    "measure-prompt.yaml",
)
SIZE_THRESHOLD = 100  # bytes
FALLBACK_TZ = "America/New_York"


REPO_ROOT: Path  # set in main(), used by all git wrappers


def git(*args: str, text: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["git", *args],
        capture_output=True, text=text, check=True, cwd=REPO_ROOT,
    )


def list_committed_paths() -> list[str]:
    out = git("log", "--all", "--diff-filter=AM", "--name-only", "--pretty=format:").stdout
    paths = set()
    for line in out.splitlines():
        p = line.strip()
        if p.startswith(".cobbler/history/"):
            paths.add(p)
    return sorted(paths)


def latest_commit_for(path: str) -> str | None:
    out = git("log", "--all", "--diff-filter=AM", "--pretty=format:%H", "-1", "--", path).stdout.strip()
    return out or None


def blob_size(commit: str, path: str) -> int:
    try:
        return int(git("cat-file", "-s", f"{commit}:{path}").stdout.strip())
    except (subprocess.CalledProcessError, ValueError):
        return 0


def blob_content(commit: str, path: str) -> bytes:
    return git("show", f"{commit}:{path}", text=False).stdout


def in_recover_scope(path: str) -> bool:
    basename = path.rsplit("/", 1)[-1]
    return any(basename.endswith(s) for s in RECOVER_SUFFIXES)


def build_prefix_to_utc(paths_with_commits: list[tuple[str, str]]) -> dict[str, pd.Timestamp]:
    out: dict[str, pd.Timestamp] = {}
    for path, commit in paths_with_commits:
        if not path.endswith("-stats.yaml"):
            continue
        m = PATH_PATTERN.match(path)
        if not m:
            continue
        prefix = m.group("prefix")
        if prefix in out:
            continue
        try:
            data = yaml.safe_load(blob_content(commit, path))
            sa = data.get("started_at") if isinstance(data, dict) else None
            if not sa:
                continue
            ts = pd.Timestamp(sa)
            ts = ts.tz_convert("UTC") if ts.tz is not None else ts.tz_localize("UTC")
            out[prefix] = ts
        except Exception:
            continue
    return out


def fallback_utc(prefix: str) -> pd.Timestamp:
    naive = datetime.strptime(prefix, "%Y-%m-%d-%H-%M-%S")
    return pd.Timestamp(naive).tz_localize(FALLBACK_TZ).tz_convert("UTC")


def find_run_id(ts: pd.Timestamp, runs: pd.DataFrame) -> str:
    mask = (runs["start_at"] <= ts) & (runs["end_at"] > ts)
    matches = runs.loc[mask]
    return str(matches.iloc[0]["run_id"]) if len(matches) > 0 else "orphan"


def main() -> int:
    global REPO_ROOT
    REPO_ROOT = Path(__file__).resolve().parent.parent.parent
    repo_root = REPO_ROOT
    runs_path = repo_root / "analysis" / "datasets" / "runs.csv"
    if not runs_path.exists():
        print(f"runs.csv not found at {runs_path}", file=sys.stderr)
        return 1
    runs = pd.read_csv(runs_path)
    runs["start_at"] = pd.to_datetime(runs["start_at"], utc=True)
    runs["end_at"] = pd.to_datetime(runs["end_at"], utc=True)

    paths = list_committed_paths()
    print(f"Found {len(paths)} distinct paths in .cobbler/history/")

    paths_with_commits = [(p, c) for p in paths if (c := latest_commit_for(p)) is not None]
    print(f"Resolved {len(paths_with_commits)} paths to commits")

    prefix_to_utc = build_prefix_to_utc(paths_with_commits)
    print(f"Built UTC map for {len(prefix_to_utc)} timestamp prefixes from stats.yaml")

    out_root = repo_root / "analysis" / "raw" / "recovered"
    out_root.mkdir(parents=True, exist_ok=True)

    counts: dict[str, int] = defaultdict(int)
    bucket_counts: dict[str, int] = defaultdict(int)
    bytes_written = 0

    for path, commit in paths_with_commits:
        m = PATH_PATTERN.match(path)
        if not m:
            counts["bad_filename"] += 1
            continue
        if not in_recover_scope(path):
            counts["out_of_scope"] += 1
            continue
        size = blob_size(commit, path)
        if size <= SIZE_THRESHOLD:
            counts["too_small"] += 1
            continue
        prefix = m.group("prefix")
        ts = prefix_to_utc.get(prefix)
        if ts is None:
            try:
                ts = fallback_utc(prefix)
                counts["used_fallback_tz"] += 1
            except Exception:
                counts["bad_timestamp"] += 1
                continue
        run_id = find_run_id(ts, runs)
        bucket_counts[run_id] += 1

        out_dir = out_root / run_id
        out_dir.mkdir(parents=True, exist_ok=True)
        (out_dir / path.rsplit("/", 1)[-1]).write_bytes(blob_content(commit, path))
        bytes_written += size
        counts["written"] += 1

    print("\nRecovery counts:")
    for k, v in sorted(counts.items(), key=lambda x: -x[1]):
        print(f"  {k:25s}  {v:5d}")
    print(f"  total bytes written       {bytes_written/1024/1024:7.2f} MB")
    print("\nFiles per run bucket:")
    for run_id, n in sorted(bucket_counts.items(), key=lambda x: -x[1]):
        print(f"  {run_id:50s}  {n:5d}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
