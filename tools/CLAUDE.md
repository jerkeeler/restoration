# tools/

Ad-hoc Python probes used when AoM ships a patch and replays start failing to
parse. **Not** shipped in the release binary, **not** required by the Go parser
or the test pipeline. They exist so the next patch break can be diagnosed in
seconds instead of rebuilt from scratch.

## What's here

| Script                       | Purpose |
| ---------------------------- | ------- |
| `decompress_inner.py`        | Strip the outer wrapper + l33t/zlib layer to produce the inner header bytes (`/tmp/inner_*.bin`). Use this when diff-walking the header tree, XMB name lists, or searching for moved UTF-16 strings (steps 3–7 of the patch-debugging playbook in `parser/CLAUDE.md`). |
| `trace_command_stream.py`    | Walk every command list in the post-l33t command-stream region of a `.mythrec`, printing the offset, type, and body length of each command. Stops at the first parse failure with a precise diagnostic (which list, which command, which sentinel mismatched). This is the tool that pinpointed the prequeueTech mid-list misalignment in May 2026. |

## How to run

Each script has a [PEP 723](https://peps.python.org/pep-0723/) inline metadata
header so `uv` resolves the (zero) dependencies and Python version automatically:

```fish
uv run tools/decompress_inner.py <replay.mythrec>
uv run tools/trace_command_stream.py <replay.mythrec>
uv run tools/trace_command_stream.py <replay.mythrec> --prequeue-tech-bytes 13
```

## Caveats

These scripts mirror Go-parser logic (notably the per-command refiner byte
lengths in `parser/gameCommands.go` and the command-list framing in
`parser/gameCommandParser.go`). **They will drift.** When you add or change a
refiner in Go, update the `REFINER_LENGTH` table in
`trace_command_stream.py`. There are no tests guarding this; the contract is
"matches the Go parser as of the last patch break." If the script disagrees with
the Go parser on a known-good replay, the script is wrong.

The patch-debugging playbook lives in the project root `CLAUDE.md` and the
`parser/CLAUDE.md`. Read those before writing a new probe — most of the
recipes you'll need (decompress once, walk the header tree, instrument
`parseGameCommand`, probe-and-detect over hard-coding) are already documented
there.
