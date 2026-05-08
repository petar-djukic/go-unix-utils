#!/usr/bin/env python3
"""Per-requirement specification metadata.

Walks `docs/specs/software-requirements/srd*.yaml`, joins each requirement
to weight/status from `.cobbler/requirements.yaml`, and to release_id from
`docs/road-map.yaml`. Emits one row per individual requirement (R1.1 etc.)
to `analysis/datasets/specs.csv`.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

import pandas as pd
import yaml

CODE_BLOCK_RE = re.compile(r"(?:^|\n)(?: {4,}[^\n]+|```)", re.MULTILINE)
TARGET_RE = re.compile(r"\b(cmd|pkg)/([A-Za-z0-9_.-]+)")


def derive_target(srd_full_id: str, srd_title: str) -> str | None:
    if srd_title:
        m = TARGET_RE.search(srd_title)
        if m:
            return f"{m.group(1)}/{m.group(2)}"
    parts = srd_full_id.split("-", 1)
    if len(parts) == 2:
        return parts[1]
    return None


def has_code_block(text: str) -> bool:
    if not text:
        return False
    return bool(CODE_BLOCK_RE.search(text))


def load_requirements_yaml(path: Path) -> dict[str, dict[str, dict]]:
    if not path.exists():
        return {}
    with path.open() as f:
        d = yaml.safe_load(f) or {}
    return d.get("requirements", {}) or {}


def load_release_map(road_map_path: Path) -> dict[str, str]:
    """Map use_case slug suffix (e.g. 'testutils') → release version (e.g. '00.0')."""
    if not road_map_path.exists():
        return {}
    with road_map_path.open() as f:
        d = yaml.safe_load(f) or {}
    out: dict[str, str] = {}
    for rel in d.get("releases", []) or []:
        version = rel.get("version")
        for uc in rel.get("use_cases", []) or []:
            uc_id = uc.get("id", "") or ""
            # uc_id format: relNN.N-uc<NNN>-<slug>
            m = re.match(r"rel[\d.]+-uc\d+-(.+)$", uc_id)
            if m and version is not None:
                out[m.group(1)] = f"rel{version}"
    return out


def walk_srd(srd_path: Path, req_state: dict[str, dict],
             release_map: dict[str, str]) -> list[dict]:
    with srd_path.open() as f:
        d = yaml.safe_load(f) or {}
    srd_full_id = d.get("id") or srd_path.stem
    srd_title = d.get("title", "") or ""
    short_id = srd_full_id.split("-", 1)[0]
    target = derive_target(srd_full_id, srd_title)

    # acceptance criteria with traces
    ac_count_per_req: dict[str, int] = {}
    for ac in d.get("acceptance_criteria", []) or []:
        for trace in ac.get("traces", []) or []:
            ac_count_per_req[trace] = ac_count_per_req.get(trace, 0) + 1

    # package_contract.exports keyed by req trace
    pc_signatures: dict[str, str] = {}
    pc = d.get("package_contract", {}) or {}
    for export in pc.get("exports", []) or []:
        sig = export.get("signature") or export.get("name") or ""
        for trace in export.get("traces", []) or []:
            pc_signatures[trace] = str(sig)

    # release_id by target slug
    release_id = None
    if target:
        slug = target.split("/", 1)[-1]
        release_id = release_map.get(slug)

    rows: list[dict] = []
    requirements = d.get("requirements", {}) or {}
    for sec_id, section in requirements.items():
        if not isinstance(section, dict):
            continue
        items = section.get("items", []) or []
        for item in items:
            if not isinstance(item, dict):
                continue
            for req_id, body in item.items():
                if not isinstance(body, dict):
                    body = {}
                text = body.get("text", "") or ""
                state_entry = req_state.get(req_id, {}) if isinstance(req_state, dict) else {}
                rows.append({
                    "srd_id": short_id,
                    "srd_full_id": srd_full_id,
                    "srd_title": srd_title,
                    "target": target,
                    "req_id": req_id,
                    "req_section": sec_id,
                    "req_text": text,
                    "text_length": len(text),
                    "has_code_block": has_code_block(text),
                    "weight": state_entry.get("weight"),
                    "state": state_entry.get("status"),
                    "release_id": release_id,
                    "acceptance_criteria_count": ac_count_per_req.get(req_id, 0),
                    "package_contract_signature": pc_signatures.get(req_id),
                })
    return rows


def main(repo_root: Path) -> int:
    spec_dir = repo_root / "docs" / "specs" / "software-requirements"
    out = repo_root / "analysis" / "datasets" / "specs.csv"

    full_state = load_requirements_yaml(repo_root / ".cobbler" / "requirements.yaml")
    release_map = load_release_map(repo_root / "docs" / "road-map.yaml")

    rows: list[dict] = []
    srd_count = 0
    for srd_path in sorted(spec_dir.glob("srd*.yaml")):
        srd_count += 1
        srd_full_id = srd_path.stem
        per_srd_state = full_state.get(srd_full_id, {}) or {}
        rows.extend(walk_srd(srd_path, per_srd_state, release_map))

    df = pd.DataFrame(rows)
    out.parent.mkdir(parents=True, exist_ok=True)
    df.to_csv(out, index=False)

    print(f"SRDs scanned:    {srd_count}")
    print(f"requirements:    {len(df)}")
    print(f"unique releases: {df['release_id'].nunique()}")
    print(f"with weight:     {int(df['weight'].notna().sum())}")
    print(f"with state:      {int(df['state'].notna().sum())}")
    print(f"with code block: {int(df['has_code_block'].sum())}")
    print(f"wrote {out}")
    return 0


if __name__ == "__main__":
    repo = Path(__file__).resolve().parents[2]
    sys.exit(main(repo))
