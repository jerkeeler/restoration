# restoration

A CLI for restoring [Age of Mythology: Retold](https://www.ageofempires.com/games/aom/age-of-mythology-retold/) .mythrec files into human readable formats, such as JSON. This is a replay parser.

## Background

This package was written so that the AoM community could have an easy to use, well-maintained tool for parsing rec files. It is also made so that [aomstats.io](https://aomstats.io) can use it to parse rec files and extract even more stats (e.g., minor god choices and eAPM).

Heavily inspired by [loggy's work](https://github.com/erin-fitzpatric/next-aom-gg/blob/main/src/server/recParser/recParser.ts) for aom.gg and his [proof of concept python parser](https://github.com/Logg-y/retoldrecprocessor/blob/main/recprocessor.py). I am unabashedly using his work as a reference to build this package. Some portions may be direct copies.

## Installation

To install the CLI you can download the latest release from the [Releases](https://github.com/jerkeeler/restoration/releases) page. The executables are created by the [release.yaml](.github/workflows/release.yaml) GitHub Actions workflow and are built for Windows, Linux, and MacOS. Note that the executables are not signed in any sort of way, so you may need to allow them explicitly via your OS's security settings. The correct permissions may also not be set. Specifically on unix based systems you may need to run `chmod +x restoration-<os>-<arch>` to make the file executable.

If you are a developer and have Go installed you can also build the CLI from source:

```bash
git clone https://github.com/jerkeeler/restoration
cd restoration
go build -o restoration
```

Or use `go install` and add the binary to your path:

```bash
go install github.com/jerkeeler/restoration
```

## Usage

restoration is a CLI tool and has good help documentation built in. For Windows users, this means you must run it from the command prompt. You can run `restoration --help` to see the available commands and flags.

```
❯ ./restoration-darwin-arm64 help
A CLI parser of Age of Mythology: Retold .mythrec files. restoration's
main utility is parsing .mythrec files and output a large JSON file of the contents
to make the .mythrec more human readable and consumeable by other applications.

Usage:
  restoration [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  parse       Parses .mythrec files to human-readable json
  rename      Renames all .mythrec (or .mythrec.gz) in a directory based on player names

Flags:
  -h, --help      help for restoration
      --is-gzip   Indicates whether the input files are compressed with gzip
  -v, --verbose   Enable verbose logging

Use "restoration [command] --help" for more information about a command.
```

The parse command help:

```
❯ ./restoration-darwin-arm64 parse --help
Parses .mythrec files to human-readable json

Usage:
  restoration parse [flags]

Flags:
  -h, --help            help for parse
  -o, --output string   Save the output JSON to the provided filepath
      --pretty-print    Pretty print the output JSON
  -q, --quiet           Quiet mode, no output to standard output
      --slim            Slim mode, don't output game commands
      --stats           Stats mode, add stats to the output, you cannot use this with slim mode

Global Flags:
      --is-gzip   Indicates whether the input files are compressed with gzip
  -v, --verbose   Enable verbose logging
```

The rename command help:

```
❯ ./restoration-darwin-arm64 rename --help
This command will rename replay files in a directory based on the player names in the .mythrec file.

Only files ending in .mthyrec (or .mythrec.gz if the is-gzip flag is set) will be renamed. All other files will
be ignored. This will override the existing files in the directory.

You can optionally provide a prefix and/or suffix that will be added to the renamed files.

Usage:
  restoration rename [directory] [flags]

Flags:
  -h, --help            help for rename
      --prefix string   Prefix to add to renamed files
      --suffix string   Suffix to add to renamed files (before the extension)

Global Flags:
      --is-gzip   Indicates whether the input files are compressed with gzip
  -v, --verbose   Enable verbose logging

```

### Example Output

Example output running the parse command in a slim mode and pretty printed:

```bash
./restoration-darwin-arm64 parse IamMagic_vs_TAG_RecoN.mythrec.gz --is-gzip --slim --pretty-print
```

```json
{
  "MapName": "alfheim",
  "BuildNumber": 512899,
  "BuildString": "AoMRT_s.exe 512899 //stream/Athens/stable",
  "ParsedAt": "2025-01-13T15:05:00.554616-05:00",
  "ParserVersion": "0.1.0",
  "GameLengthSecs": 1381.1,
  "GameSeed": 31019,
  "WinningTeam": 0,
  "GameOptions": {
    "gameaivsai": false,
    "gameallowaiassist": false,
    "gameallowcheats": false,
    "gameallowtitans": true,
    "gameblockade": false,
    "gameconquest": false,
    "gamecontrolleronly": false,
    "gamefreeforall": false,
    "gameismpcoop": false,
    "gameismpscenario": false,
    "gamekoth": false,
    "gameludicrousmode": false,
    "gamemaprecommendedsettings": false,
    "gamemilitaryautoqueue": false,
    "gamenomadstart": false,
    "gameonevsall": false,
    "gameregicide": false,
    "gamerestored": false,
    "gamerestrictpause": true,
    "gamermdebug": false,
    "gamestorymode": false,
    "gamesuddendeath": false,
    "gameteambalanced": false,
    "gameteamlock": true,
    "gameteamsharepop": false,
    "gameteamshareres": false,
    "gameteamvictory": false,
    "gameusedenforcedagesettings": false
  },
  "Players": [
    {
      "PlayerNum": 1,
      "TeamId": 0,
      "Name": "IamMagic",
      "ProfileId": 1073764190,
      "Color": 1,
      "RandomGod": false,
      "God": "Gaia",
      "Winner": true,
      "EAPM": 118.07979147056695,
      "MinorGods": ["Oceanus", "Theia", "Atlas"]
    },
    {
      "PlayerNum": 2,
      "TeamId": 1,
      "Name": "TAG_RecoN",
      "ProfileId": 1073796204,
      "Color": 2,
      "RandomGod": false,
      "God": "Zeus",
      "Winner": false,
      "EAPM": 77.19933386431106,
      "MinorGods": ["Athena", "Apollo", "Hera"]
    }
  ],
  "GameCommands": null
}
```

Game commands will be a long list that looks like the following when not in slim mode:

```json
{
  "GameCommands": [
    {
      "GameTimeSecs": 8.35,
      "CommandType": "build",
      "PlayerNum": 1,
      "Value": "EconomicGuild"
    },
    {
      "GameTimeSecs": 18.9,
      "CommandType": "build",
      "PlayerNum": 2,
      "Value": "Storehouse"
    },
    {
      "GameTimeSecs": 21,
      "CommandType": "godPower",
      "PlayerNum": 1,
      "Value": "GaiaForest"
    },
    {
      "GameTimeSecs": 22.05,
      "CommandType": "research",
      "PlayerNum": 1,
      "Value": "Pickaxe"
    },
    {
      "GameTimeSecs": 128.6,
      "CommandType": "autoqueue",
      "PlayerNum": 2,
      "Value": "VillagerGreek"
    },
    {
      "GameTimeSecs": 143.2,
      "CommandType": "train",
      "PlayerNum": 2,
      "Value": "Jason"
    }
  ]
}
```

## Limitations

1. There is no automated testing, currently, and there may be many bugs.
2. Currently this tool only supports 1v1 games, team games are in the workds.
3. This only works for multiplayer games and has only been tested on ranked game replays.
4. Not all command types are currently paresd and stored in the output JSON, if you have a request for a specific command type please open an issue.
5. There are currently no stats calculations and a bunch of metadata flags

## Roadmap

- [ ] Add support for team games
- [ ] Add refiners for all command types
- [ ] Add stats calculation
- [ ] Add testing

## Development, contribution guidelines

**Guidelines:**

- Only `go fmt` go code will be accepted
- If you are adding a new command, ensure that sufficient documentation is added
- A general rule of thumb is that a `parse*` function is working on the underlying byte slice and everything else is using data that has been marshalled into a coherent data structure
- Anyone is open to contributing to this repo, just open a PR and I will review it
- Always use `slog` and never use `fmt.Println`
  - Be liberal with `slog.Debug`
- Keep the output from the `parse` command clean, it should only be JSON. Ideally one can then take the standard output and pipe it into a file or any other tool (such as `jq`).
  - For example you could get the mapname and winners using this jq string: `jq '{map: .MapName, players: [.Players[] | {name: .Name, winner: .Winner}]}' test.json`
