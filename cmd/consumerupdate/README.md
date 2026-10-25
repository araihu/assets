# Consumer asset updates

`consumerupdate` prepares an allowlisted patch from a GitHub Assets release.
Consumer repositories own `araihu-assets.json`, including the current six-field
release receipt and source-to-destination mappings. Visual mappings require an
exact catalog `canonicalName`; the two published Arai Hu CSS themes can be
selected directly by path.

```text
consumerupdate --manifest araihu-assets.json --event-file event.json \
  --token-file /run/secrets/github-token --output-dir /out
```

The event file is GitHub's `repository_dispatch` event with action
`araihu-assets-released`, or a `workflow_dispatch` event containing the same six
metadata fields in `inputs`. Event data cannot replace mappings.

The tool compares the supplied receipt with the uploaded GitHub release, commit,
checksums, and archive identity. It then verifies the bounded archive, inventory,
catalog, and selected file sizes and hashes before writing a fresh output
directory. It accepts catalog schemas v1 and v2. Archive links, traversal,
duplicate paths, and invalid output mappings fail before output is written.

Export `/out` onto the consumer checkout, then create a PR restricted to
`araihu-assets.json` and the mapped destinations. Keep the tool source pinned to
a reviewed Assets commit and provide its GitHub read token only after building.
