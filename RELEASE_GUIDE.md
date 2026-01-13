# MocaChain Release Guide

## **Overview**

This project uses <a href="https://goreleaser.com/">GoReleaser</a> with [GitHub Actions](https://github.com/features/actions)
for automated releases.

Include disk space optimizations to handle large CGO builds across multiple architectures.

## **Create a Release**

There are two ways to create a release:

### Option 1. Create and push a tag

Must be in the format `v<MAJOR>.<MINOR>.<PATCH>`.

See [Version Tagging Convention](#version-tagging-convention) for more information on how to tag a release and the different
types of releases such as pre-releases and release candidates.

```bash
# Example: pre-release tag
git tag -a v1.0.1-alpha.1 -m "Pre-Release v1.0.1-alpha.1"
git push origin v1.0.1-alpha.1
```

OR, for a production release:

```bash
# Example: production release tag
git tag -a v1.0.1 -m "Production Release v1.0.1"
git push origin v1.0.1
```

### Option 2. Create a release via the GitHub UI

Alternatively, releases can also be created manually via the GitHub UI and goreleaser
will be automatically triggered by the new tag.

1. Navigate to the Releases page in the GitHub UI.
2. Click the "Draft a new release" button.
3. Enter the release name and description and create a new tag.
4. Click the "Publish release" button.

## **What Happens When a versioned Tag is Pushed or a Release is Created**

With either method, GitHub Actions will automatically:

1. ✅ Free up ~25GB of disk space by removing unused tools
2. ✅ Build binaries for Darwin (amd64, arm64) and Linux (amd64, arm64) with CGO
3. ✅ Create archives with mainnet and testnet config files
4. ✅ Generate checksums
5. ✅ Build multi-arch Docker images for linux/amd64 and linux/arm64
6. ✅ Push images to GitHub Container Registry (ghcr.io)
7. ✅ Create a GitHub release with all artifacts

You can now manually update the release notes in the GitHub UI if needed.

## **What Gets Released**

### Binaries

- `mocad_darwin_x86_64.tar.gz` - macOS Intel
- `mocad_darwin_arm64.tar.gz` - macOS Apple Silicon
- `mocad_linux_x86_64.tar.gz` - Linux AMD64
- `mocad_linux_arm64.tar.gz` - Linux ARM64
- `checksums.txt` - SHA256 checksums

Each archive includes:

- Pre-compiled `mocad` binary
- `mainnet_config/` directory with configuration files
- `testnet_config/` directory with configuration files

### Docker Images

**Multi-arch manifests** (automatically selects correct architecture):

- `ghcr.io/mocachain/mocad:v1.0.0`
- `ghcr.io/mocachain/mocad:latest`

**Architecture-specific images**:

- `ghcr.io/mocachain/mocad:v1.0.0-amd64`
- `ghcr.io/mocachain/mocad:v1.0.0-arm64`
- `ghcr.io/mocachain/mocad:latest-amd64`
- `ghcr.io/mocachain/mocad:latest-arm64`

> **Note**: Docker image paths use lowercase repository owner (`mocachain` not `MocaChain`) per Docker registry requirements.

## **Using Released Artifacts**

### Option 1. Binary Installation

Download the appropriate archive for your platform

```bash
wget https://github.com/mocachain/moca/releases/download/v1.0.1/mocad_darwin_arm64.tar.gz
```

Extract

```bash
tar -xzf mocad_darwin_arm64.tar.gz

```

Move binary to PATH

```bash
sudo mv bin/mocad /usr/local/bin/
```

Verify

```bash
mocad version
```

### Option 2. Docker Usage

Pull the image (automatically selects correct architecture)

```bash
docker pull ghcr.io/mocachain/mocad:v1.0.1
```

Or use latest

```bash
docker pull ghcr.io/mocachain/mocad:latest
```

Run

```bash
docker run -d \
 --name mocad \
 -p 26656:26656 \
 -p 26657:26657 \
 -p 1317:1317 \
 -p 9090:9090 \
 -p 8545:8545 \
 -p 8546:8546 \
 ghcr.io/mocachain/mocad:v1.0.1 \
 start --home /root/.mocad
```

## **Security Notes**

- ✅ The PAT is stored as an encrypted secret
- ✅ It's only accessible during workflow runs
- ✅ It's not exposed in logs or artifacts
- ✅ Use a token with minimal required scopes (`repo` for private repos)
- ✅ Docker images are signed with GitHub's OIDC token
- ⚠️ Never commit tokens to the repository
- ⚠️ Rotate tokens periodically for security
- ⚠️ Review PAT access logs regularly

## **Version Tagging Convention**

- **Format**: `v{MAJOR}.{MINOR}.{PATCH}[-{PRERELEASE}]`
- **Examples**:
  - `v1.0.0` - **Production release** (tagged automatically as `latest`)
  - `v1.0.1-alpha` - Alpha pre-release (NOT tagged automatically as `latest`)
  - `v1.0.2-beta.1` - Beta pre-release (NOT tagged automatically as `latest`)
  - `v2.0.0-rc.1` - Release candidate (NOT tagged automatically as `latest`)

**Important**: Only stable releases (without suffix) receive the `latest` Docker tag automatically.
Pre-releases are marked as such in GitHub and do not update the `latest` tag automatically.

The "v" prefix is required and included in all Docker image tags.
