from dataclasses import replace

import duckdb
import pytest

from plpred.config import load
from plpred.db import connect


def test_load_resolves_paths_from_repo_root():
    cfg = load()
    assert cfg.db_path.is_absolute()
    assert cfg.db_path.parts[-2:] == ("data", "pl.duckdb")
    assert cfg.first_season <= cfg.current_season


def test_write_then_read(tmp_path):
    cfg = replace(load(), db_path=tmp_path / "t.duckdb")
    with connect(read_only=False, cfg=cfg) as con:
        con.execute("CREATE TABLE t (x INTEGER)")
        con.execute("INSERT INTO t VALUES (1), (2)")
    with connect(cfg=cfg) as con:
        assert con.execute("SELECT sum(x) FROM t").fetchone()[0] == 3


def test_read_only_rejects_writes(tmp_path):
    cfg = replace(load(), db_path=tmp_path / "t.duckdb")
    with connect(read_only=False, cfg=cfg) as con:
        con.execute("CREATE TABLE t (x INTEGER)")
    with connect(cfg=cfg) as con, pytest.raises(duckdb.Error):
        con.execute("INSERT INTO t VALUES (1)")


def test_missing_db_fails_loudly(tmp_path):
    cfg = replace(load(), db_path=tmp_path / "nope.duckdb")
    with pytest.raises(FileNotFoundError):
        with connect(cfg=cfg):
            pass