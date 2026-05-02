# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Walk the command-log section of a .mythrec, printing per-list and per-command
offsets, types, and body lengths. Stops at the first parse failure with a
precise diagnostic.

Mirrors the Go parser's command-stream walker
(parser/gameCommandParser.go::parseCommandList + parseGameCommand) and the
per-refiner byte lengths from parser/gameCommands.go. **Will drift** —
re-sync REFINER_LENGTH when a refiner changes. See tools/CLAUDE.md.

Usage:
    uv run tools/trace_command_stream.py <replay.mythrec>
    uv run tools/trace_command_stream.py <replay.mythrec> --quiet
    uv run tools/trace_command_stream.py <replay.mythrec> --prequeue-tech-bytes 13

The --prequeue-tech-bytes knob lets you A/B test alternate body lengths for
command type 72 (prequeueTech) when a patch shifts its layout.
"""

import argparse
import gzip
import struct
import sys
from pathlib import Path

# Refiner byte lengths, sourced from parser/gameCommands.go (Refine methods).
# Re-check these against the source of truth when adding to it. Commands marked
# "var" compute their length from a probe and need a special case below.
REFINER_LENGTH = {
    0: 44,    # task: 4 int32 + vector + float + 3 int32
    1: 12,    # research: 3 int32
    2: 18,    # train: 4 int32 + 2 int8
    3: 52,    # build
    4: 32,    # setGatherPoint: 2 int32 + vector + float + 2 int32
    7: 9,    # delete: 2 int32 + int8
    9: 8,    # stop: 2 int32
    12: 57,   # useProtoPower
    13: 20,   # marketBuySell
    14: 8,    # ungarrison: 2 int32
    16: 21,   # resign: 5 int32 + int8
    18: 12,   # unknown 18: 3 int32
    19: 25,   # tribute: 4 int32 + 2 float + int8
    23: 18,   # finishUnitTransform: 4 int32 + 2 int8
    25: 14,   # setUnitStance: 2 int32 + 2 int8 + int32
    26: 13,   # changeDiplomacy: 2 int32 + int8 + int32
    34: 8,    # townBell
    35: 12,   # autoScoutEvent
    37: 13,   # changeControlGroup: 2 int32 + int8 + int32
    38: 12,   # repair: 3 int32
    39: 12,   # unknown 39: 3 int32
    41: 45,   # taunt
    44: 16,   # cheat
    45: 20,   # cancelQueuedItem: 5 int32
    48: 16,   # setFormation
    53: 12,   # startUnitTransform: 3 int32
    55: 20,   # unknown 55: 2 int32 + vector
    66: 12,   # autoqueue
    67: 9,    # toggleAutoUnitAbility: 2 int32 + int8
    68: 32,   # timeShift
    69: 36,   # buildWallConnector: 3 int32 + 2 vector
    71: 12,   # seekShelter: 3 int32
    72: 16,   # prequeueTech (build >= 601511); --prequeue-tech-bytes overrides
    73: 8,    # unknown 73: 2 int32
    75: 16,   # prebuyGodPower
    78: -1,   # unknown 78: 20 or 28 depending on probe (handled below)
}


def u16(d: bytes, o: int) -> int:
    return struct.unpack_from("<H", d, o)[0]


def u32(d: bytes, o: int) -> int:
    return struct.unpack_from("<I", d, o)[0]


def parse_game_command(data, offset, prequeue_bytes):
    """Mirror parseGameCommand. Returns (next_offset, type, info, start)."""
    start = offset
    cmd_type = data[offset + 1]
    ten = offset
    offset += 10
    offset += 20 if cmd_type == 14 else 8

    three_off = offset
    if u32(data, offset) != 3:
        return None, cmd_type, f"three!=3 at 0x{three_off:x} (got {u32(data, three_off)})", start
    offset += 4

    if cmd_type == 19:
        offset += 4
    else:
        if u16(data, offset) != 1:
            return None, cmd_type, f"one!=1 at 0x{offset:x}", start
        offset += 4
        pid = u16(data, offset)
        if pid > 12:
            return None, cmd_type, f"playerId={pid}", start
        offset += 4
    offset += 4

    nu = u16(data, offset); offset += 4 + 4 * nu
    nv = u16(data, offset); offset += 4 + 12 * nv
    npa = 13 + u16(data, offset); offset += 4 + npa

    body_start = offset
    if cmd_type == 78:
        bl = 28 if u32(data, offset + 12) == 3 else 20
    elif cmd_type == 72:
        bl = prequeue_bytes
    elif cmd_type in REFINER_LENGTH and REFINER_LENGTH[cmd_type] > 0:
        bl = REFINER_LENGTH[cmd_type]
    else:
        return None, cmd_type, f"unregistered command type {cmd_type}", start

    return offset + bl, cmd_type, f"body=0x{body_start:x}+{bl}", start


def parse_command_list(data, offset, prequeue_bytes):
    list_start = offset
    et = u32(data, offset); offset += 5  # entryType + earlyByte
    if et & 225 != et:
        return None, f"bad entryType={et}", []
    if et & 96 == 96:
        return None, f"96 entryType={et}", []
    offset += 1 if et & 1 else 4

    log = [f"list@0x{list_start:x} et={et}"]
    if et & 96 != 0:
        if et & 32:
            ni = data[offset]; offset += 1
        else:
            ni = u32(data, offset); offset += 4
        for i in range(ni):
            no, ct, info, cs = parse_game_command(data, offset, prequeue_bytes)
            if no is None:
                return None, f"cmd #{i+1} @0x{cs:x} type={ct}: {info}", log
            log.append(f"  cmd#{i+1} @0x{cs:x} type={ct} -> 0x{no:x} ({info})")
            offset = no

    if et & 128:
        ni = data[offset]; offset += 1 + 4 * ni

    eb = data[offset]; offset += 1 + eb
    unk = data[offset]; offset += 1
    if unk == 0:
        offset += 8
    elif unk != 1:
        return None, f"unk={unk}", log
    f4 = u16(data, offset); offset += 4 + 4 * f4
    eidx = u32(data, offset); offset += 4
    fb = data[offset]
    if fb != 0:
        return None, f"finalByte={fb} at 0x{offset:x}", log
    return offset + 1, None, log


def find_command_stream_start(data: bytes, command_offset: int) -> int:
    """Mirror parseGameCommands' header-end → footer search → -19 walkback."""
    footer_idx = data.find(b"\x19\x00\x00\x00", command_offset)
    if footer_idx < 0:
        raise SystemExit("FOOTER not found after command_offset")
    return footer_idx - 19


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("replay", type=Path)
    ap.add_argument("--gz", action="store_true")
    ap.add_argument("--quiet", action="store_true",
                    help="only print first/last few lists and the failure")
    ap.add_argument("--prequeue-tech-bytes", type=int, default=16,
                    help="body length for command type 72 (default: 16)")
    args = ap.parse_args()

    raw = args.replay.read_bytes()
    if args.gz or args.replay.suffix == ".gz":
        raw = gzip.decompress(raw)

    # commandCount = uint32 at offset 23 of the (gunzipped) outer file
    command_count = u32(raw, 23)
    sv_idx = raw.find(b"\x73\x76")
    command_offset = u32(raw, sv_idx + 2)
    start = find_command_stream_start(raw, command_offset)
    print(f"commandCount={command_count} commandOffset=0x{command_offset:x} stream-start=0x{start:x}")

    offset = start
    n = 0
    while offset < len(raw):
        next_off, err, log = parse_command_list(raw, offset, args.prequeue_tech_bytes)
        if err is not None:
            print(f"\nFAIL at list #{n+1} (offset 0x{offset:x}): {err}")
            for line in log:
                print(line)
            sys.exit(1)
        if not args.quiet:
            for line in log:
                print(line)
        offset = next_off
        n += 1
        if n >= command_count:
            break

    print(f"\nOK — parsed {n} command lists, ended at offset 0x{offset:x} "
          f"(file size 0x{len(raw):x})")


if __name__ == "__main__":
    main()
