"""Load config.yaml, the config shared by Go and Python"""
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

# python/plpred/config.py -> parents[2] is the repo root
REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_CONFIG = REPO_ROOT / "config.yaml"

@dataclass(frozen=True)
class Config:
    root: Path
    db_path: Path
    models_dir: Path
    sql_dir: Path
    first_season: int
    current_season: int
    walk_forward_test_seasons: int
    raw: dict[str, Any] # the rest of the YAML file

def load(path: Path | str = DEFAULT_CONFIG) -> Config:
    path = Path(path).resolve()
    with path.open() as f:
        raw = yaml.safe_load(f)

    root = path.parent
    return Config(
        root=root,
        db_path = root / raw["db"]["path"],
        models_dir= root / raw["paths"]["models_dir"],
        sql_dir= root / raw["paths"]["sql_dir"],
        first_season=raw["seasons"]["first"],
        current_season=raw["seasons"]["current"],
        walk_forward_test_seasons=raw["model"]["walk_forward_test_seasons"],
        raw=raw,
    )