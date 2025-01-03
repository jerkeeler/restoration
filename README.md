# Restoration

# **UNDER ACTIVE DEVELOPMENT** - This project is currently under active development and is not yet ready for use.

[![PyPI version](https://badge.fury.io/py/restoration.svg)](https://badge.fury.io/py/restoration)

Restore [Age of Mythology](https://www.ageofempires.com/games/aom/age-of-mythology-retold/) rec files into a human readable format (and other utilities).

## Background

This package was written so that the AoM community could have an easy to use, well-maintained tool for parsing rec files. It is also made so that [aomstats.io](https://aomstats.io) can use it to parse rec files and extract some even more stats (e.g., minor god choices and build orders).

Heavily inspired by [loggy's work](https://github.com/erin-fitzpatric/next-aom-gg/blob/main/src/server/recParser/recParser.ts) for aom.gg and his [proof of concept python parser](https://github.com/Logg-y/retoldrecprocessor/blob/main/recprocessor.py). I am unabashedly using his work as a reference to build this package. Some portions may be direct copies.

## Development setup

Clone the respository, create a new virtual environment, install the package and its dependencies:

```bash
# Install the package and its dev dependencies
pip install -e .[dev]

# Setup pre-commit hooks
pre-commit install
```

## Publishing

```bash
# Install the build and twine packages
pip install build twine

# Setup .pypirc file with correct tokens
# Run the publish script to publish to pypi
./bin/publish.sh
```
