# Plugin release guide

This guide explains how plugin author*innen publish plugin binaries as GitHub Releases so the registry automation can pick them up and add them to `plugins.json`.

## Overview

The publish flow is intentionally simple:

1. Build your plugin binaries for all supported platforms.
2. Create a GitHub Release in [`SemRels/go-semrel-plugins`](https://github.com/SemRels/go-semrel-plugins).
3. Upload every platform binary plus its matching `.sha256` checksum file.
4. The registry GitHub Actions workflow discovers the release, validates the metadata, and regenerates `plugins.json`.
5. Consumers can then install the version from the registry index.

```mermaid
flowchart LR
    A[Create tag\n{plugin-name}-v{semver}] --> B[Publish GitHub Release\nin go-semrel-plugins]
    B --> C[Upload binaries\n+ .sha256 files]
    C --> D[Registry GitHub Actions sync]
    D --> E[Validate metadata\nagainst plugin schema]
    E --> F[Update plugins.json]
```

The most important rule is consistency: the plugin name in the release tag, binary filenames, checksum files, and registry metadata must all match.

## Release naming convention

Use this format for every Git tag and GitHub Release:

```text
{plugin-name}-v{semver}
```

Examples:

- ✅ `provider-github-v0.1.0`
- ✅ `changelog-generator-v1.2.3`
- ❌ `v0.1.0` - missing plugin name
- ❌ `github-release` - missing version

Recommendations:

- keep `plugin-name` identical to the plugin `name` used in registry metadata
- use lowercase kebab-case names such as `provider-github`
- use valid SemVer versions such as `0.1.0`, `1.2.3`, or `1.3.0-rc.1`

## Binary asset naming

Name every uploaded binary like this:

```text
{plugin-name}-{os}-{arch}
```

On Windows, add the `.exe` extension:

```text
{plugin-name}-windows-{arch}.exe
```

Supported platforms:

| Platform | Filename example |
| --- | --- |
| `linux-amd64` | `provider-github-linux-amd64` |
| `linux-arm64` | `provider-github-linux-arm64` |
| `darwin-amd64` | `provider-github-darwin-amd64` |
| `darwin-arm64` | `provider-github-darwin-arm64` |
| `windows-amd64` | `provider-github-windows-amd64.exe` |
| `windows-arm64` | `provider-github-windows-arm64.exe` |

Additional guidance:

- upload one binary per supported platform
- do not rename assets after checksums were generated
- keep the filename prefix identical to the release tag prefix

## Checksum generation

Every binary must have a matching `.sha256` file.

Examples:

### Linux

```bash
sha256sum provider-github-linux-amd64 > provider-github-linux-amd64.sha256
```

### macOS

```bash
shasum -a 256 provider-github-darwin-arm64 > provider-github-darwin-arm64.sha256
```

### Windows PowerShell

```powershell
(Get-FileHash provider-github-windows-amd64.exe -Algorithm SHA256).Hash | Set-Content provider-github-windows-amd64.exe.sha256
```

Generate checksum files for every asset and upload both files:

- `provider-github-linux-amd64`
- `provider-github-linux-amd64.sha256`
- `provider-github-darwin-arm64`
- `provider-github-darwin-arm64.sha256`
- `provider-github-windows-amd64.exe`
- `provider-github-windows-amd64.exe.sha256`

A combined checksum manifest such as `checksums.txt` is optional and useful for humans, but it does not replace the per-binary `.sha256` files.

## Release body / metadata

Use the release body to document the context a plugin user or registry maintainer needs:

- **Changelog** - what changed in this version?
- **Compatibility** - minimum required go-semrel version and the supported Proto/gRPC version
- **Breaking changes** - call them out explicitly if behavior or configuration changed incompatibly
- **Installation notes** - optional setup or migration steps

A good release body answers these questions quickly:

- Is this version safe to upgrade to?
- Which go-semrel versions can load it?
- Does it require a new plugin protocol / gRPC generation?
- Do users need to change configuration after upgrading?

## Example: step-by-step guide

```bash
# 1. Tag erstellen (lokal)
git tag -a provider-github-v0.1.0 -m "Release provider-github v0.1.0"

# 2. Build für alle Plattformen (Makefile/Script)
make build-all-platforms  # Erzeugt: provider-github-{os}-{arch}

# 3. Checksums generieren
for file in \
  provider-github-linux-amd64 \
  provider-github-linux-arm64 \
  provider-github-darwin-amd64 \
  provider-github-darwin-arm64 \
  provider-github-windows-amd64.exe \
  provider-github-windows-arm64.exe; do
  sha256sum "$file" > "$file.sha256"
done
sha256sum \
  provider-github-linux-amd64 \
  provider-github-linux-arm64 \
  provider-github-darwin-amd64 \
  provider-github-darwin-arm64 \
  provider-github-windows-amd64.exe \
  provider-github-windows-arm64.exe > checksums.txt

# 4. Push & Release (gh CLI)
git push origin provider-github-v0.1.0
gh release create provider-github-v0.1.0 \
  provider-github-linux-amd64 \
  provider-github-linux-arm64 \
  provider-github-darwin-amd64 \
  provider-github-darwin-arm64 \
  provider-github-windows-amd64.exe \
  provider-github-windows-arm64.exe \
  provider-github-linux-amd64.sha256 \
  provider-github-linux-arm64.sha256 \
  provider-github-darwin-amd64.sha256 \
  provider-github-darwin-arm64.sha256 \
  provider-github-windows-amd64.exe.sha256 \
  provider-github-windows-arm64.exe.sha256 \
  checksums.txt \
  --title "Provider GitHub v0.1.0" \
  --notes "Your changelog here"

# 5. Verify in Registry
# Nach ~6h sollte das Plugin in plugins.json sichtbar sein
```

If you use PowerShell instead of Bash, the included [`../scripts/release.ps1`](../scripts/release.ps1) script can be used as a starting template.

## Troubleshooting

### The release does not appear in the registry

Check the common causes first:

- the sync job has not run yet - wait for the next scheduled registry sync
- the release tag does not match `{plugin-name}-v{semver}`
- one or more binary assets are missing or use the wrong filename
- a `.sha256` file is missing or does not match the uploaded binary
- the release was published as a prerelease and your consumer ignores prereleases

### How to debug

- inspect the generated registry index on GitHub Pages: `https://semrels.github.io/go-semrel-registry/plugins.json`
- compare the release asset names against the naming rules in this guide
- download a binary and its `.sha256` file and recompute the checksum locally
- check the registry repository GitHub Actions workflow logs for validation or aggregation errors

## FAQ

### What is a prerelease?

A prerelease is an unstable version such as `1.0.0-rc.1` or a GitHub Release marked as prerelease. Use it for preview builds; consumers may choose to ignore prereleases by default.

### Can I update my plugin later?

Yes. Publish a new release with a new tag such as `provider-github-v0.1.1`. The registry keeps version history and the newest valid release will be aggregated on the next sync.

### Do I need my own repository, or can I publish in `go-semrel-plugins`?

For the GitHub Releases flow documented here, publish plugin releases in [`SemRels/go-semrel-plugins`](https://github.com/SemRels/go-semrel-plugins). If a separate-repository workflow is introduced later, the registry documentation will be updated accordingly.

## Need help?

- start with the [submission overview](contribute.md)
- see the repository-wide [contributing guide](../CONTRIBUTING.md)
- if you are publishing from `go-semrel-plugins`, also check its `CONTRIBUTING.md` for the cross-repository link back to this guide
