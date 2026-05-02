# CLAUDE.md

Onboarding notes for Claude / AI agents and human contributors working in this repo. Keep it tight; update when something here goes stale.

## What this is

`restoration` is a CLI that parses Age of Mythology: Retold replay files (`.mythrec`, optionally gzipped as `.mythrec.gz`) into JSON. The replay file is a binary format produced by the game itself; this tool decompresses it, walks its node tree, and decodes the player command log so the data can be inspected, archived, or fed into stats services like aomstats.io.

The format is **not officially documented**. Knowledge of it is reverse-engineered, partly inherited from prior community work (notably loggy's Python proof-of-concept and the next-aom-gg TS parser — see README). This repo does **not** claim to understand every byte of a replay or every command type; see "Known unknowns" below.

## Layout

```
main.go                 # thin entry: cmd.Execute()
cmd/                    # Cobra CLI (root, parse, rename, version)
parser/                 # all replay-decoding logic — see parser/CLAUDE.md
bin/build.sh            # cross-compile for linux/darwin/windows
.github/workflows/      # release workflow, triggers on `v*` tag push
releases/               # build output (gitignored)
```

The CLI is intentionally thin — `cmd/parse.go` validates the path and calls `parser.ParseToJson`. Almost all complexity lives in `parser/`.

## Subcommands

| Command   | What it does                                                |
| --------- | ----------------------------------------------------------- |
| `parse`   | Parse a single replay file → JSON to stdout or `-o` path    |
| `rename`  | Walk a directory and rename replays based on parsed players |
| `version` | Print parser version (sourced from `parser.VERSION`)        |

Global flags: `--is-gzip` (file is gzipped), `-v/--verbose` (debug logging).
`parse` flags: `-o/--output`, `-q/--quiet`, `--pretty-print`, `--slim` (drop game commands), `--stats` (per-player aggregates; mutually exclusive with `--slim`).

The contract for `parse`: **stdout is pure JSON.** All logging goes through `log/slog` to stderr so `restoration parse … | jq …` always works. Don't break this — no `fmt.Println` for diagnostics.

## Build, run, format

```bash
go build -o restoration                  # local dev binary
go run . parse path/to/file.mythrec.gz --is-gzip --slim --pretty-print
bin/build.sh                              # cross-compile all release targets to releases/
go fmt ./...                              # required before commits
go vet ./...                              # recommended sanity check
```

There is no test suite yet (see "Known unknowns"). The release workflow (`.github/workflows/release.yaml`) runs on `v*` tag pushes, builds for linux-amd64 / darwin-amd64 / darwin-arm64 / windows-amd64, and attaches binaries to a GitHub Release.

When bumping the parser version, update `parser/consts.go` (`VERSION`) and tag the commit `vX.Y.Z` to trigger the release.

## Dependencies

- Go 1.23.4
- `github.com/spf13/cobra` — CLI framework

That's the entire external surface. Stdlib handles compression (`compress/gzip`, `compress/zlib`), encoding (`encoding/binary`, `encoding/json`), UTF-16, etc. Keep the dependency footprint small.

## Conventions

- `go fmt` is canonical — tabs, idiomatic Go.
- Use `log/slog` for all logging. Be liberal with `slog.Debug`. Never `fmt.Println` for diagnostics — it leaks into the JSON stream.
- Naming convention in `parser/`: a `parse*` function operates on the raw `[]byte` slice; everything else operates on already-decoded structs. Preserve this split when adding code.
- Errors propagate up to the CLI layer, which prints to stderr and exits non-zero. Don't `os.Exit` from inside the parser.
- Conventional commits in commit messages (e.g. `fix:`, `feat:`, `chore:`).

## Known unknowns (read this before changing the parser)

The replay format is reverse-engineered. Several things are explicitly unfinished:

- **Not all game commands are decoded.** Commands 18, 39, 55, 73, 78 are recognized (their byte length is known well enough to advance the cursor) but their payloads are not extracted. Many other command IDs may exist in the wild and will cause the parser to **fail hard** — see `parser/gameCommandParser.go` where an unregistered command type returns an error with no fallback.
- **Discovering a new command is empirical.** The workflow is: launch the game, perform the action you want to identify, save the replay, diff it against a baseline replay, and locate the new bytes. Hardcoded byte offsets and length heuristics in `parser/gameCommands.go` are the result of this process. Patches to the game can shift these.
- **Team game support is partial.** Winner detection assumes 1v1 (`Winner = !losingTeam`). FFA and team games are not fully validated.
- **No automated tests.** No fixture replays are checked in. Verify changes by parsing real replay files locally and inspecting the JSON diff.
- **Some header/XMB structure is still mysterious.** See TODOs in `parser/header.go` (e.g. the `kL` node) and `parser/gameCommands.go` (cheat name lookup, command 78 length heuristic).

When AoM ships a patch, the parser may break. Recent commit history is dominated by post-patch fixes (commands 73/78, AI player parsing, node misrecognition). Treat the parser as a living artifact that tracks the live game.

## Verifying a change

There is no `go test ./...` to lean on. After touching `parser/`:

1. Parse a known-good replay with `--pretty-print` and compare the JSON to a saved baseline.
2. If the change touches game commands, parse with `--stats` too and sanity-check counts (units trained, techs researched, EAPM) against expectations from the actual game.
3. If a player has reported a broken replay, parse that file specifically and confirm it now succeeds.
4. `go vet ./...` and `go fmt ./...` before committing.

## Debugging a new-patch parsing failure

When AoM ships a patch and replays from the new build start failing, the workflow that has worked well is:

### Ask up front for the right inputs

Before digging in, ask the user (or yourself) for:

- **One known-good replay from the previous patch** — this is the single most valuable artifact. It anchors "what the format used to look like" so you can diff against it byte-for-byte. Without it, you're reverse-engineering in the dark.
- **More than one failing replay from the new patch.** Different games surface different bugs:
  - A short / fast-resign game has very few commands and may parse "successfully" while masking deeper issues (a 2-second resign in this codebase hit only `autoqueue` + `resign`, hiding a `prequeueTech` byte-length change that broke longer games).
  - A full multi-age game with research, prequeue, god powers, builds, trains gives broad coverage of command-type code paths.
  - Two new replays let you confirm a structural change is reproducible (same `mU` offset preamble, same byte sentinels) rather than file-specific noise.

Surface this to the user proactively: "Got the failing replay — do you also have one from the previous patch that *did* work, and ideally a longer non-resign new replay too? Comparing them is much faster than working from the failing file alone."

### The diagnostic ladder

Run these in order; each rules out a layer of the format:

1. **Reproduce the error and read it literally.** The parser surfaces structural violations (`x1 not equal to 12632`, `entryIdx was not sequential`, `refiner not defined for commandType=N`) — those are precise pointers to *where* in the format the assumption broke. Don't paper over them.
2. **Check the file is what you think it is.** `file <path>` and `xxd | head` confirm gzip vs raw vs new wrapper. Magic bytes: `1f 8b` = gzip, `6c 33 33 74` = `l33t`, the `RG` outer wrapper has lived inside replays for a while now.
3. **Decompress to inner bytes once.** Save them under `/tmp/inner_<TAG>.bin` for the failing file *and* the known-good one. All subsequent analysis uses these.
4. **Diff outer headers and BG roots.** If they're byte-identical for the first ~256 bytes, layout changes are deeper than the wrapper — go to step 5. If they differ, your bug is structural in the wrapper.
5. **Walk the header tree (Python, mirroring `parser/header.go` `parseTree`/`findTwoLetterSeq`).** Print children of `BG/GM/GD`, `BG/MP/ST`, `BG/J1/PL`. Histogram the child tokens. A new token (in this session: `mU`) appearing only in the new file is a strong lead.
6. **Diff XMB name lists.** Iterate `BG/GM/GD/gd` children, registering single-file and multi-file entries the same way `parseXmbMap` does. Compare old vs new sets — this is how we found `proto` had been removed.
7. **Search for known strings as UTF-16 LE in the new inner data.** If a unit/tech/god name from the old format still exists in the new file, you can locate where it was moved to (and check whether it's still XMB-shaped at that location).
8. **For command-stream desyncs, instrument and bisect.** Add a temporary `slog.Debug` of `commandType` and `offset` at the top of `parseGameCommand`, run with `-v`, capture stdout. Look for the last successful command before the failing iteration; dump raw bytes from that command's offset against the same command in the old file. The byte position where they diverge is your fix.
9. **Prefer probe-and-detect over hard-coding patches.** Where possible, fix by detecting the format dynamically (e.g., the `prequeueTech` 13-vs-16-byte fix probes for the `00 01 19` footer envelope at both candidate positions instead of switching on build number). Self-detecting code keeps OLD replays working without threading build numbers through the parser. Only fall back to a build-number switch if no reliable signal is available.
10. **Verify on every replay you have, including OLD.** Diff the OLD JSON output before and after the fix — it must be unchanged. Then check the new replays produce the human-meaningful fields the user cares about (map, players, gods, minor gods).

### Things to remember about this codebase

- `parseXmb` is **structure-driven**, not size-bounded. It will happily read past a containing node's reported size if the XMB internally claims more children. This is a feature, not a bug — it's how the new `mU` proto catalog (which spills past `mU.size` into bytes the header walk treats as sibling `XN` nodes) gets parsed correctly.
- `findTwoLetterSeq` scans up to 50 bytes for the next valid ASCII pair, so non-ASCII padding inserted between header nodes is naturally tolerated.
- `Decompressl33t` finds the `l33t` magic via `bytes.Index`, so any leading wrapper bytes are silently skipped.
- The `kL` token is explicitly skipped during header parsing (`header.go`); add similar skips for any new bogus token if needed.
- A graceful-fallback helper (`parseXmbIfPresent` in `formatter.go`, `protoName` in `gameCommands.go`) is the standard pattern for "XMB might be missing/renamed" — keep using it for new lookups.

## Pointers to deeper docs

- `parser/CLAUDE.md` — internals of the parser package: format anatomy, file roles, decoding pipeline, gotchas.
- `README.md` — user-facing docs and example output. Update this when changing CLI flags or output shape.
