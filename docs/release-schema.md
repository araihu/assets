# Release schema

`release.json` inventories one immutable Arai Hu Assets release. It uses schema
version 1 and records the SemVer release tag, identity revision, runtime
compatibility version, SHA-256 hashes of `catalog.json`, `themes.json`, and
`campaigns.json`, and each pre-release file's path, SHA-256, and byte size.

Inventory files place the three release documents first in catalog, themes,
campaigns order; remaining paths are lexical, unique, relative, and safe.
`release.json` never inventories itself. The build writes it only after the
three documents exist, then writes `checksums.txt` and the release archives.

Public cumulative assembly copies every verified member below
`releases/<tag>/`. An existing identical member is accepted; differing bytes,
symbolic links, and non-regular files fail assembly.
