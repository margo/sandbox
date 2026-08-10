# Release Process

This document is for **release managers and maintainers**, outlining versioning strategy, release workflow, and deliverables for the Sandbox.

The 'Sandbox' release corresponds to a specific version of the MARGO Specification, including the OpenAPI swagger definition of the specification, as mentioned in the release notes.

The release may contain SUPs (features) or bug-fixes / improvement suggestions raised by the community, The release notes will have a list of submissions which are part of the release.

**For developers**: See [CONTRIBUTING.md](../CONTRIBUTING.md) for commit standards, branch naming, and PR workflow.

## Scope and Release Strategy

**Key principle**: All sandbox components receive **a single version number** per release. Any codebase change in (device agent, WFM, shared-lib) results in a version bump for the entire repository. This ensures clear dependency management and simplified compatibility tracking.

**Note**: This process applies to the sandbox repo only. The Margo spec project has its own versioning cycle and can be tracked in its respective repository.

---

## Versioning

We follow [Semantic Versioning (SemVer)](https://semver.org/): `MAJOR.MINOR.PATCH`

**Developer commits** follow [Conventional Commits](https://conventionalcommits.org/) format (see [CONTRIBUTING.md](../CONTRIBUTING.md#commit-style)):

| Change Type | Version Bump | When |
|-------------|--------------|------|
| Breaking changes | MAJOR | Any commit with `BREAKING CHANGE:` footer |
| New features | MINOR | `feat` commits |
| Bug fixes | PATCH | `fix` commits or dependency updates |
| Docs/chore | No bump | Documentation or non-dependency maintenance |

**When multiple commit types exist**, highest priority wins: `BREAKING CHANGE` > `feat` > `fix/chore(deps)` > `docs`

Example: `1.0.0` + feat + fix + docs → `1.1.0` (MINOR takes precedence)

### Release Types

**Stable Release** (`v1.2.3`):
- Production-ready, tested, immutable once released
- Tagged from `main` → CI builds and publishes artifacts

**Release Candidate** (`v1.2.3-rc.1`):
- Pre-release for community testing during code freeze
- Published to GHCR with full tag (e.g., `ghcr.io/margo/device-agent:v1.2.3-rc.1`)
- Community decides when to promote RC to stable (typically 2+ weeks of testing)

**Nightly Build** (`nightly`):
- Automated daily snapshot from `development` branch
- For development/testing only, **NOT for production**
- Tagged as `ghcr.io/margo/device-agent:nightly` (overwritten daily)
- Only latest nightly retained

## Tools and Automation

| Tool | Purpose | Config | Notes |
|------|---------|--------|-------|
| **GoReleaser** | Build binaries, images, checksums, release notes | [.goreleaser.yaml](.goreleaser.yaml) | Triggered on `v*` tags |
| **GHCR** | Docker image registry | Built-in | Multi-platform manifest support |
| **CommitLint** | Validate conventional commits in CI | `.commitlint.config.js` | Blocks non-compliant PRs |
| **GoSec** | Security scan on source code | Built-in | Runs in CI pipeline |
| **Go Test** | Golang based unit/integration tests | Built-in | N/A |

**CI Pipelines**:
- **Release Pipeline**: Triggered on `git tag v*` (automatic GoReleaser build + publish)
- **Nightly Pipeline**: Daily schedule against `development` branch (tagged `nightly`)

## Testing and Verification

- Deploy to staging; verify its working
- Collect community testing feedback via Discourse and GitHub Issues
- If critical issues found: Create hotfix release or rollback

## Release Workflow

The typical workflow for preparing a release is as follows:

### Phase 1: Code Freeze & Stabilization (1-2 weeks)

1. **Declare Code Freeze**: Announce code freeze on the `development` branch via discussion channels (e.g., Discourse, Teams, GitHub Discussions). Specify the freeze window (typically 1-2 weeks).

2. **Stabilization Criteria**: During the freeze window, the release is considered stable when:
   - All test cases pass (unit, integration, end-to-end, and conformance tests)
   - Test coverage meets the defined threshold
   - All critical and high-priority bugs are resolved
   - Community feedback on `-rc.N` versions is addressed (if applicable)

3. **Hotfixes During Freeze**: See the [Immutable Releases / Hotfix Procedure](#immutable-releases) section below for the full hotfix workflow.

4. **Who Decides Stabilization**: Release lead (determined by designated maintainer or CODEOWNERS) confirms when freeze criteria are met and stabilization is complete.

### Phase 2: Release & Tagging

5. **Merge to Main**: Merge all approved changes from `development` into `main`. Ensure all tests pass in CI.

6. **Create Tag**: Create a Git tag from `main` following SemVer (e.g., `git tag v1.2.3`).
   - Tag format: `v{MAJOR}.{MINOR}.{PATCH}` for stable, `v{MAJOR}.{MINOR}.{PATCH}-{prerelease}` for RCs.

7. **Push Tag & Trigger Release**: Push the tag to the repository (see Roles and Responsibilities for who performs this).
   - CI pipeline automatically triggers on tag push (no manual trigger needed)
   - GoReleaser builds binaries, Docker images, checksums, changelog and generates release notes

8. **Publish Artifacts**:
   - Binary artifacts and checksums are published to GitHub Releases.
   - Docker images are pushed to GHCR with version tags and platform-specific tags.

### Phase 3: Post-Release

9. **Create Discussion Thread**: Open a release discussion thread in Discourse for the Margo community to:
   - Provide feedback and suggestions.
   - Ask questions about the release.

10. **Continue Development**: Meanwhile, development continues on the `development` branch.

11. **Announce Release**: Notify the community via Teams, Discourse, mailing lists, or other communication channels with highlights and/or release notes.

### Automation Implementation Details

- Releases are automated using [GoReleaser](https://goreleaser.com/) triggered by tags matching `v*` in [.github/workflows/release.yml](.github/workflows/release.yml).
- GoReleaser handles building binaries, docker images, checksums, and changelogs as defined in [.goreleaser.yaml](.goreleaser.yaml).

## Immutable Releases

**Principle**: Once a version is released, artifacts cannot be modified. This ensures reproducibility and security.

**Hotfix Procedure** (for critical bugs discovered during code freeze or post-release):
1. Create `hotfix/` branch from `main` (the commit tagged as the previous release)
2. Fix the issue, test thoroughly
3. Merge hotfixes to both `main` and `development`
4. During code freeze: Each hotfix will result in an RC increment (e.g., `v1.2.0-rc.1` → `v1.2.0-rc.2`)
5. Post-release: Tag a new patch version (e.g., `v1.2.1`) and release
6. Never re-upload or modify artifacts from the original release

**Emergency Pre-releases** (rare): For critical security issues, release as RC (e.g., `v1.2.1-rc.1`) with accelerated testing and community consensus.

## Release Checklist

### Pre-Release (Code Freeze)
- [ ] Code freeze announced with dates and criteria
- [ ] All features for this release merged to `development`
- [ ] All tests pass (unit, integration, e2e)
- [ ] Code coverage as defined in this document
- [ ] GoSec security scan passes
- [ ] No critical bugs open
- [ ] Helm deployment tested on staging
- [ ] Release lead approves readiness

### Release Execution
- [ ] Merge `development` → `main` with PR review
- [ ] CI checks pass on `main`
- [ ] Tag created (`v1.2.3`) from `main`
- [ ] Tag pushed (triggers CI automatically)
- [ ] GoReleaser completes successfully
- [ ] Binary artifacts + checksums verified
- [ ] Docker images pushed to GHCR
- [ ] Helm chart version updated
- [ ] Release notes generated
- [ ] SBOM generated and attached
- [ ] GitHub Release page published

### Post-Release
- [ ] Release announced via Discourse/Teams/channels
- [ ] Community feedback collected
- [ ] Monitor for critical issues

## Deliverables

Each release produces the following artifacts, published to GitHub Releases and GHCR:

### 1. Executable Binaries
- **Device Agent** binary for multiple platforms:
  - Linux amd64 (x86-64)
  - Linux arm64
- **Format**: ELF executables
- **Checksums**: SHA256 checksums provided for each binary
- **Location**: GitHub Releases page

### 2. Container Images
- **Multi-platform Docker images** built for platforms:
  - amd64 (x86-64 architecture)
  - arm64 (ARM 64-bit architecture)
- **Registry**: GitHub Container Registry (GHCR)
- **Tag examples**:
  - Release: `ghcr.io/margo/device-agent:v1.2.3`
  - Release candidate: `ghcr.io/margo/device-agent:v1.2.3-rc.1`
  - Nightly: `ghcr.io/margo/device-agent:nightly` (continuously updated)
- **Manifest**: Multi-platform manifests are created automatically by Docker buildx to allow `docker pull` to fetch the correct platform.

### 3. Release Notes
- **Auto-generated** summary of changes from commits since the previous release
- **Content**: List of features, fixes, breaking changes, and dependency updates. Documentation-only commits are excluded.
- **Location**: GitHub Releases page

### 4. Changelog
- **Auto-generated** by GoReleaser from conventional commits

### 5. Software Bill of Materials (SBOM)
- **Format**: SPDX
- **Location**: GitHub Releases attachment

## Backwards Compatibility & Deprecation

**Within Major Versions**: Code written for `v2.0.0` should work for all the minor/patch versions like `v2.3.5` without changes.

**Breaking Changes** (requires MAJOR bump):
- API schema changes
- Config file format changes
- Binary input/output format changes
- Include clear migration guide in release notes (with code examples and rollback instructions), if Margo agrees on providing the backwards compatibility.

## Communication

### Release Announcements

- **Timing**: Announce after release is published (artifacts available in GitHub Releases and GHCR)
- **Channels**:
  - [Discourse](https://discourse.margo.org/)
  - Teams
  - Other Margo Community channels
- **Content**:
  - Link to GitHub Release page

### Support & Feedback

- **Issue reporting**: Community members report issues in GitHub Issues
- **Discussion response**: Maintainers respond promptly to Discourse and Github Issues

## Roles and Responsibilities

| Role | Who | Responsibilities |
|------|-----|------------------|
| **Maintainers** | Dev Team (See CODEOWNERS) | Approve PRs, declare freeze, assert readiness, push tags, monitor release |
| **QA/Community** | Dev Team (See CODEOWNERS) + SUP Owners | Execute tests, report bugs, verify RCs, smoke tests |
| **DevOps/Release Manager** | Dev Team (See CODEOWNERS) | Maintain workflows, push tags, verify artifacts, manage nightly builds |

**Release Lead** (per CODEOWNERS, @ajcraig, @phil-abb): Confirms code freeze criteria met and signs off on stabilization readiness

See [CONTRIBUTING.md](../CONTRIBUTING.md) for developer responsibilities (commit standards, testing, PR process).

---

### Steps currently not covered

- Minimum test coverage check in the pipeline
- SBOM generation
- HELM chart and docker-compose as part of release artifacts
- Section for `security and compliances strategy` for the release pipeline
- Signing of release artifacts
