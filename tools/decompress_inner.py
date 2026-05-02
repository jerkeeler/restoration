# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Decompress the inner l33t/zlib layer of a .mythrec replay to raw bytes.

Outputs to /tmp/inner_<basename>.bin by default, or to --out if specified.
Useful for header-tree walking, XMB diffing, and UTF-16 string searches when
a new AoM patch breaks parsing. See parser/CLAUDE.md for the playbook.

Note: this is NOT the same as the byte stream the command-log walks against —
the Go parser walks the OUTER (only-gzip-stripped) bytes for the command log,
not this inner-decompressed buffer. Use this for header / XMB analysis only.
"""

import argparse
import gzip
import struct
import sys
import zlib
from pathlib import Path


def decompress_l33t(data: bytes) -> bytes:
    idx = data.find(b"l33t")
    if idx < 0:
        raise SystemExit("no l33t magic found")
    # 4 bytes magic + 4 bytes uncompressed-size, then zlib stream
    return zlib.decompress(data[idx + 8 :])


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("replay", type=Path, help="path to .mythrec or .mythrec.gz")
    ap.add_argument("--gz", action="store_true", help="treat input as gzip")
    ap.add_argument("--out", type=Path, default=None)
    args = ap.parse_args()

    raw = args.replay.read_bytes()
    if args.gz or args.replay.suffix == ".gz":
        raw = gzip.decompress(raw)

    inner = decompress_l33t(raw)
    out = args.out or Path("/tmp") / f"inner_{args.replay.stem}.bin"
    out.write_bytes(inner)
    print(f"wrote {len(inner):,} bytes ({len(inner):#x}) to {out}")


if __name__ == "__main__":
    main()
