#!/usr/bin/env python3
"""Check a sparse German difference overlay against its Global comparison report."""
import hashlib
import json
import pathlib
import sys


def fail(message):
    raise SystemExit(f"Invalid German overlay: {message}")


def main(global_dir, german_dir):
    report = json.loads((german_dir / "german_r2_diff.json").read_text(encoding="utf-8"))
    summary = report["summary"]
    differences = report["different"]
    if (report["reference_version"], report["scanned_version"]) != ("steam_r2_global", "steam_r2_german"):
        fail("wrong source editions")
    if any(summary[k] for k in ("missing", "extra", "errors")) or summary["different"] != len(differences) or summary["global_files"] != summary["german_files"]:
        fail("incomplete comparison report")
    if sum(p.is_file() for p in global_dir.rglob("*")) != summary["global_files"]:
        fail("Global file count differs from the comparison report")
    actual = {p.relative_to(german_dir).as_posix() for p in german_dir.rglob("*") if p.is_file() and p.name != "german_r2_diff.json"}
    expected = {item["path"] for item in differences}
    if actual != expected or len(expected) != len(differences):
        fail(f"overlay paths differ from report: unlisted={sorted(actual - expected)}, missing={sorted(expected - actual)}")
    for item in differences:
        for directory, relative, field in ((global_dir, item["global_path"], "global"), (german_dir, item["path"], "german")):
            if pathlib.PurePosixPath(relative).is_absolute() or ".." in pathlib.PurePosixPath(relative).parts:
                fail(f"unsafe report path: {relative}")
            path = directory / relative
            if not path.is_file() or path.is_symlink():
                fail(f"missing or linked file: {path}")
            digest = hashlib.sha256()
            with path.open("rb") as stream:
                for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                    digest.update(chunk)
            if path.stat().st_size != item[field]["size"] or digest.hexdigest() != item[field]["sha256"].lower():
                fail(f"size or SHA-256 mismatch: {path}")
    print(f"Verified {len(differences)} German differences against Global; unchanged files inherit Global sources.")


if __name__ == "__main__":
    try:
        main(pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]))
    except (IndexError, KeyError, ValueError, OSError) as exc:
        fail(str(exc))
