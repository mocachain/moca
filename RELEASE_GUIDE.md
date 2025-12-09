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
3. ✅ Upload raw binaries (no archives)
4. ✅ Generate checksums
5. ✅ Build multi-arch Docker images for linux/amd64 and linux/arm64
6. ✅ Push images to GitHub Container Registry (ghcr.io)
7. ✅ Create a GitHub release with all artifacts

You can now manually update the release notes in the GitHub UI if needed.

## **What Gets Released**

### Binaries

Raw binaries organized by platform:

- `darwin_amd64/mocad` - macOS Intel
- `darwin_arm64/mocad` - macOS Apple Silicon
- `linux_amd64/mocad` - Linux AMD64
- `linux_arm64/mocad` - Linux ARM64
- `checksums.txt` - SHA256 checksums

### Docker Images

**Multi-arch manifests** (automatically selects correct architecture):

- `ghcr.io/sledro/mocad:v1.0.0`
- `ghcr.io/sledro/mocad:latest`

**Architecture-specific images**:

- `ghcr.io/sledro/mocad:v1.0.0-amd64`
- `ghcr.io/sledro/mocad:v1.0.0-arm64`
- `ghcr.io/sledro/mocad:latest-amd64`
- `ghcr.io/sledro/mocad:latest-arm64`

## **Using Released Artifacts**

### Option 1. Binary Installation

Download the appropriate binary for your platform

```bash
# macOS Apple Silicon
wget https://github.com/sledro/moca/releases/download/v1.0.1/darwin_arm64/mocad

# macOS Intel
wget https://github.com/sledro/moca/releases/download/v1.0.1/darwin_amd64/mocad

# Linux AMD64
wget https://github.com/sledro/moca/releases/download/v1.0.1/linux_amd64/mocad

# Linux ARM64
wget https://github.com/sledro/moca/releases/download/v1.0.1/linux_arm64/mocad
```

Make executable and move to PATH

```bash
chmod +x mocad
sudo mv mocad /usr/local/bin/
```

Verify

```bash
mocad version
```

### Option 2. Docker Usage

Pull the image (automatically selects correct architecture)

```bash
docker pull ghcr.io/sledro/mocad:v1.0.1
```

Or use latest

```bash
docker pull ghcr.io/sledro/mocad:latest
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
 ghcr.io/sledro/mocad:v1.0.1 \
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
