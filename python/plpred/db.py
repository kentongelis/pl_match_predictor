"""DuckDB connections. Read-only by default"""
from collections.abc import Iterator
from contextlib import contextmanager

import duckdb

from plpred.config import Config, load


@contextmanager
def connect(
    read_only: bool = True, cfg: Config | None = None
    ) -> Iterator[duckdb.DuckDBPyConnection]:
    cfg = cfg or load()
    if read_only and not cfg.db_path.exists():
        raise FileNotFoundError(
            f"database not found at {cfg.db_path}; run `plpred clean` first"
        )
    con = duckdb.connect(str(cfg.db_path), read_only=read_only)
    try:
        yield con
    finally:
        con.close()