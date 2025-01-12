#!/usr/bin/env bash

GOOS=linux GOARCH=amd64 go build -o releases/myrecparser-linux-amd64
GOOS=darwin GOARCH=amd64 go build -o releases/myrecparser-darwin-amd64
GOOS=darwin GOARCH=arm64 go build -o releases/myrecparser-darwin-arm64
GOOS=windows GOARCH=amd64 go build -o releases/myrecparser-windows-amd64.exe
