---
title: "Install"
description: "Install the capcom binary with Homebrew, Scoop, winget, go install or a release archive, and check it runs."
sidebar: { order: 1 }
---

capcom is a single binary with no runtime dependencies. It runs inside a git repository, so `git`
must be installed too. Pick whichever channel suits your platform.

## Homebrew (macOS, Linux)

```sh
brew install airiclenz/tap/capcom
```

`brew upgrade capcom` picks up later releases.

## Scoop (Windows)

```sh
scoop bucket add airiclenz https://github.com/airiclenz/scoop-bucket
scoop install capcom
```

`scoop update capcom` picks up later releases.

## winget (Windows)

```sh
winget install AiricLenz.Capcom
```

`winget upgrade AiricLenz.Capcom` picks up later releases.

## From source with Go

With Go 1.25 or later:

```sh
go install github.com/airiclenz/capcom/cmd/capcom@latest
```

The binary lands in `$(go env GOPATH)/bin` (or `$GOBIN` when it is set); make sure that directory
is on your `PATH`.

## Release archives

Every [GitHub release](https://github.com/airiclenz/capcom/releases) carries prebuilt archives
for Linux, macOS and Windows on amd64 and arm64:

- `capcom_<version>_<os>_<arch>.tar.gz` for Linux and macOS
- `capcom_<version>_<os>_<arch>.zip` for Windows

Each archive holds one directory, `capcom_<version>_<os>_<arch>/`, with the `capcom` binary
(`capcom.exe` on Windows), `LICENSE` and `README.md`. The release's `SHA256SUMS` file lists the
checksum of every archive; check yours before unpacking, from the directory holding both files:

```sh
grep capcom_<version>_linux_amd64.tar.gz SHA256SUMS | sha256sum -c -
# on macOS:
grep capcom_<version>_darwin_arm64.tar.gz SHA256SUMS | shasum -a 256 -c -
```

On Windows, `Get-FileHash <archive>` in PowerShell prints the hash to compare with the line in
`SHA256SUMS`.

Then copy the binary into a directory on your `PATH`.

## Check the install

```sh
capcom version
```

It prints `capcom` followed by the version number of the binary you installed. `capcom --help`
lists every command, and `capcom <command> --help` a command's flags and examples.

## A shorter command

If `capcom` is too long to type, a shell alias such as `alias cc=capcom` works well. Don't install
a binary or symlink named `cc`, though: that name belongs to the system C compiler, and build tools
call it.

## Next

The [quick start](quickstart.md) sets capcom up in a repository and walks through one Issue from
creation to close.
