# parser/CLAUDE.md

Internals of the `parser` package. The root `CLAUDE.md` covers project-level context — start there if you haven't.

## What this package does

Takes a `.mythrec` file path, decompresses it, walks its binary node tree, parses the embedded XMB metadata files and the player command log, and returns a JSON-serializable `ReplayFormatted` struct. Nothing in here is concurrency-safe; nothing in here knows about flags or the CLI.

## Public API

```go
parser.ParseToJson(replayPath string, prettyPrint, slim, stats, isGzip bool) (string, error)
parser.Parse(replayPath string, slim, stats, isGzip bool) (ReplayFormatted, error)
parser.RenameRecFiles(dir string, isGzip bool, prefix, suffix string) error
parser.VERSION  // const, kept in consts.go
```

`ParseToJson` is `Parse` + `json.Marshal`. The CLI calls `ParseToJson`; programmatic consumers can call `Parse` directly to skip the JSON round-trip.

## File roles

| File                    | Responsibility                                                                                    |
| ----------------------- | ------------------------------------------------------------------------------------------------- |
| `parser.go`             | Top-level orchestration: `Parse`, `ParseToJson`, profile-key reader                               |
| `decoder.go`            | Low-level binary primitives: int/float/bool/vector/string readers, gzip + l33t decompression      |
| `header.go`             | Parses the recursive node tree at the front of the file (the "header")                            |
| `xmb.go`                | Parses XMB (binary XML) blobs embedded in the header                                              |
| `gameCommandParser.go`  | Walks the command-log section, splits it into per-tick batches, dispatches to refiners            |
| `gameCommands.go`       | One refiner per known command type — produces typed `RawGameCommand` values                       |
| `formatter.go`          | Lazy-decodes XMB lookup tables and turns raw structs into the human-readable `ReplayFormatted`    |
| `stats.go`              | Per-player aggregates (unit counts, EAPM timeline, etc.) when `--stats` is on                     |
| `renamer.go`            | Implements `restoration rename`: walks a dir, parses each replay, renames using player names      |
| `types.go`              | All shared structs: `ReplayFormatted`, `ReplayPlayer`, `ReplayGameCommand`, `Node`, `XmbNode`, …  |
| `consts.go`             | Magic constants: `VERSION`, footer bytes, root-node token, set of nodes with substructure         |

Convention: `parse*` functions operate on raw `[]byte`. Everything else operates on already-decoded structs. Preserve this split.

## Replay file anatomy

```
[ optional gzip ]
  └─ [ l33t-compressed (zlib) blob ]
       ├─ Header
       │     Recursive tree of nodes. Each node = 2-byte ASCII token + uint32 length + payload.
       │     Nodes in NODES_WITH_SUBSTRUCTURE ({BG, J1, PL, BP, MP, GM, GD}) contain children;
       │     others are leaves. Profile keys (player names, map, seed, game options) live here.
       │     Root token is "BG".
       ├─ XMB blocks
       │     Embedded under GM/GD nodes. Binary XML files: civs, techtree, proto, powers, etc.
       │     They're the lookup tables that map numeric IDs in commands → human-readable names.
       │     Their offsets are mapped up front; their contents are parsed lazily in formatter.go.
       └─ Command log
             A sequence of per-tick batches (game runs at 20 Hz, so 1 batch = 0.05 s).
             Each batch contains 0..N commands. Each command has a type byte, player id,
             selected units, source vectors, and a type-specific payload.
             FOOTER bytes (0x19 00 00 00) demarcate batch boundaries.
```

All multi-byte integers are little-endian. Strings are UTF-16 LE prefixed by a uint16 char count and 2 bytes of padding (see `readString` in `decoder.go`).

## Pipeline

```
Parse
 ├─ os.ReadFile
 ├─ DecompressGzip      (if --is-gzip)
 ├─ Decompressl33t      (always; the inner zlib layer)
 ├─ parseHeader         → Node tree
 ├─ parseXmbMap         → map[string]XmbFile (offsets only, not parsed)
 ├─ parseProfileKeys    → game metadata (players, map, options, seed)
 ├─ parseGameCommands   → []CommandList (raw, typed game commands)
 └─ formatRawDataToReplay
      ├─ lazy parseXmb on civs / techtree / proto / powers
      ├─ build ReplayPlayers (god, color, minor gods, EAPM, winner)
      ├─ stringify each command via XMB lookups → []ReplayGameCommand
      └─ if stats: calcStats → per-player aggregates and per-minute timelines
```

The XMB tables are large; `formatRawDataToReplay` parses them once and shares them across all command-formatting calls. Don't move XMB parsing back into `parseHeader` — it materially slows batch processing.

## Game commands

Each command type has an entry in the factory built by `BuildCommandFactory()` in `gameCommands.go`. An entry is `{ Name, Refine }`:

- `Refine` reads command-specific bytes after a fixed header and returns a typed `RawGameCommand` (e.g. `TaskCommand`, `ResearchCommand`, `BuildCommand`).
- The typed command implements `Format(FormatterInput) (ReplayGameCommand, bool)` to produce the user-facing form (with IDs resolved to names via XMB).
- The `affectsEAPM` flag on `BaseCommand` decides whether the command is counted toward player EAPM. Some "spammy" commands (gather points, auto-scout, control groups, wall connectors, transform starts) are intentionally excluded.

### Currently registered command types

See the table at `gameCommands.go` `BuildCommandFactory()` for the authoritative list. Roughly: 0 task, 1 research, 2 train, 3 build, 4 setGatherPoint, 7 delete, 9 stop, 12 useProtoPower, 13 marketBuySell, 14 ungarrison, 16 resign, 19 tribute, 23 finishUnitTransform, 25 setUnitStance, 26 changeDiplomacy, 34 townBell, 35 autoScoutEvent, 37 changeControlGroup, 38 repair, 41 taunt, 44 cheat, 45 cancelQueuedItem, 48 setFormation, 53 startUnitTransform, 66 autoqueue, 67 toggleAutoUnitAbility, 68 timeShift, 69 buildWallConnector, 71 seekShelter, 72 prequeueTech, 75 prebuyGodPower. Plus opaque-payload entries: 18, 39, 55, 73, 78.

### Adding or fixing a command type

There are no test fixtures. The empirical loop:

1. Reproduce the action in-game and save the replay.
2. Parse with `-v` (verbose). If the parser errors with `"refiner not defined for commandType=N"`, you've found a new ID.
3. Diff bytes around that command against a known one of similar shape.
4. Add a `Refine` function and a factory entry. If you don't yet understand the payload, register it as an opaque `UnknownN` (consume bytes, don't extract semantics — see existing `Unknown18`/`Unknown39`/`Unknown55`/`Unknown73`/`Unknown78` for the pattern).
5. Re-parse other replays to confirm you haven't broken anything.

When a refiner advances the cursor by a constant (e.g. `offset += 20`), comment why if it's at all unobvious. Several of these are already mysterious; don't make it worse.

### Fragile spots to know about

- `Unknown78` reads a byte at `offset+12` to decide between 20 and 24-byte length — pure heuristic.
- Formation IDs are hardcoded `0=line, 1=box, 2=spread`; anything else logs a warning and becomes `"unknown"`.
- Resource IDs only recognize 1=wood, 2=food. Gold / favor / etc. fall through to `"unknown"` — likely incomplete.
- `cheatCommand` stores the cheat ID but does not yet resolve the name (TODO in `gameCommands.go`).
- `header.go` has a `kL` node it explicitly skips with a TODO. Purpose unknown.
- Winner detection assumes 1v1: `Winner = !losingTeams[teamId]`. Team-game correctness is best-effort.

## Things that will break the parser hard

- A new game command ID in the wild that isn't registered → returns an error with no recovery.
- An XMB file with an unexpected magic / version → the strict checks in `xmb.go` panic the parser.
- An AoM patch that shifts byte offsets in command payloads → silent wrong values until someone notices.

When debugging a "this replay won't parse" report, run with `-v`, find the first failure offset, and bisect against a working replay from a similar game mode and patch version.

## Patch-debugging playbook (worked example: build 601511, May 2026)

The root `CLAUDE.md` has the general workflow. This section is the codebase-specific patterns that came out of fixing build 601511, where two simultaneous breakages were hidden behind a single error message.

### What changed and where

| Change | Old (≤600762) | New (≥601511) | Where in the parser |
| ------ | ------------- | ------------- | ------------------- |
| Unit catalog location | `BG/GM/GD/gd` named `proto` | `BG/GM/GD/mU` with 14-byte preamble (`00 00 01 67 64 3d 78 58 00 00 01 00 00 00`), then standard X1 XMB | `xmb.go::parseXmbMap` registers `mU.offset + DATA_OFFSET + 14` as `xmbMap["proto"]` |
| `prequeueTech` byte length | 13 bytes (8 sentinel + 4 techId + 1 flag) | 16 bytes (8 sentinel + 4 techId + 4-byte field) | `gameCommands.go::PrequeueTechCommand.Refine` probes for the post-command `00 01 19` footer envelope at +13 and +16, picks the matching length |

These were both real and load-bearing for the user's stated goal (minor gods on a real game). The unit-catalog fix alone only got short / instant-resign replays working; the prequeueTech length fix was needed for any game with age-up prequeues — which is most non-trivial games.

### Why two replays mattered

- The first failing replay was a 2-second resign. With `proto` gone, parsing crashed on XMB magic. After fixing that, the file parsed cleanly: 2 commands, no minor gods (player resigned in age 1).
- A second failing replay (long, multi-age) was needed to expose the prequeueTech byte-length change. The first replay had no `prequeueTech` commands at all, so the bug was invisible.
- Always ask for both a long new replay *and* a known-good old replay before declaring "this format hasn't changed in <area>." Confidence comes from cross-replay consistency.

### Debug techniques that paid off

1. **Decompress once to `/tmp/inner_*.bin`, work in Python.** Reproducing `parseTree` / `parseXmbMap` / `parseXmb` in ~30 lines of Python lets you iterate on hypotheses in seconds without rebuilding Go. The Python parser doesn't need to be production-quality — it's a probe.
2. **Histogram the children of `BG/GM/GD`.** OLD had `gd: 67`. NEW had `gd: 66, XN: 382, mU: 1`. The new `mU` token plus the disappearance of one `gd` was the entire structural diff.
3. **Set-diff the XMB name lists.** Computing `OLD_names \ NEW_names` (and the reverse) immediately surfaced "exactly `proto` was removed" — no other XMB renames or additions.
4. **Search inner bytes for known UTF-16 strings.** `VillagerGreek`/`Hoplite`/`TownCenter` showed up dozens of times in the new file, proving the catalog hadn't moved out of the replay — it had relocated. Then matching positions to header-tree node ranges pinpointed `mU` as the new home.
5. **For desyncs, instrument `parseGameCommand` with `slog.Debug("commandType", commandType, "offset", offset)` and run `-v`.** Histogram the resulting log to learn which command types ran successfully and which one is sitting on the boundary at the failure. In our case the log said: 86 task, 32 autoScout, … 1 prequeueTech (the failing one). That immediately scoped the fix to one command type.
6. **Probe-and-detect beats hard-coded version checks** when the format change has a reliable byte-level signature. The `prequeueTech` fix detects the new layout by probing for the universal command-list footer envelope `00 01 19` at the two candidate offsets — works on both old and new replays without threading the build number through every refiner.
7. **`parseXmb` reads through node boundaries.** Don't trust `mU.size`; the proto XMB physically extends past it. The header walk reports 382 spurious `XN` siblings of `mU` — those are just the tail of the proto XMB. As long as `parseXmb` is anchored at `mU.offset + 20`, it reads them correctly.

### Verification recipe used here

```bash
# the three-replay matrix
go run . parse ~/Downloads/<NEW1>.mythrec --slim --pretty-print  # short fast-resign
go run . parse ~/Downloads/<NEW2>.mythrec --pretty-print          # long multi-age
go run . parse ~/Downloads/<OLD>.mythrec  --pretty-print          # last patch, must be unchanged

# OLD must be byte-identical to its pre-fix output (modulo timestamp)
diff <(jq -S 'del(.ParsedAt)' /tmp/old_pre.json) <(jq -S 'del(.ParsedAt)' /tmp/old_post.json)
```

The user-facing pass criteria for any patch fix: map name, player names, gods, **minor gods**, and a non-zero command count with no `"unknown"` payloads on a long game. Minor gods are the canary — they require both `techtree` XMB lookups *and* successful command-stream parsing through age-up prequeues, so when minor gods are right, you've usually got the rest right too.

## Adding stats

`stats.go` builds `ReplayStats` per player from the formatted command list. It buckets timelines into 1-minute slots via `math.Ceil(gameTimeSecs/60)`. To add a new aggregate:

1. Add the field to `ReplayStats` (or `Timelines`) in `types.go`.
2. Populate it in `calcStats` / `calcTotals` / `calcTimelines`.
3. Verify via `restoration parse … --stats --pretty-print` against a known replay.

Keep stats deterministic — same replay must produce byte-identical JSON across runs.

## Don'ts

- Don't `fmt.Println` from anywhere in `parser/` — it pollutes stdout, breaking `parse | jq`. Use `slog`.
- Don't `os.Exit` from inside the parser. Return errors.
- Don't add concurrency to `Parse` itself without a measured reason; the caller (e.g. `RenameRecFiles`) parallelizes across files instead.
- Don't introduce dependencies for things `encoding/binary`, `compress/*`, `unicode/utf16` already cover.
