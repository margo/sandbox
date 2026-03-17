# Release Process

This document outlines the comprehensive release process for the project, including versioning, workflow, and deliverables. It applies to the following components:
- Device agent
- CLI
- Shared libraries
- Helm charts

## Versioning

We follow [Semantic Versioning (SemVer)](https://semver.org/) for all releases. Versions are in the format `MAJOR.MINOR.PATCH`, where:

- **MAJOR**: Incremented for breaking changes (incompatible changes).
- **MINOR**: Incremented for new features (backwards-compatible additions).
- **PATCH**: Incremented for bug fixes (backwards-compatible fixes).

Version increments are determined by "conventional commit" types:
- `feat` commits → Increment MINOR.
- `fix` commits → Increment PATCH.
- Commits with `BREAKING CHANGE` → Increment MAJOR.

Example transitions:
- `1.0.0` + `feat: add new endpoint` → `1.1.0`
- `1.1.0` + `fix: resolve bug` → `1.1.1`
- `1.1.1` + `feat: change API (BREAKING CHANGE)` → `2.0.0`

### Pre-release and Build Metadata ??? Needs review!!

Additional labels can extend the format:

Use pre-release labels for releases(candidates) not ready for production, such as during stabilization or hotfixes.

### Development Suffix

While in active development (not preparing a release), we might go for nightly builds with tags like `-nightly`, e.g., `1.2.3-nightly`. This indicates the version is unstable and not for production use. TODO: This is pending.

## Release Workflow

The typical workflow for preparing a release is as follows:

1. **Declare Code Freeze**: Announce code freeze on the `develop` branch via discussion channels (e.g., Teams TWG Main). Freeze window should be 1-2 weeks to stabilize.
2. **Merge Changes**:
  - Merge all approved changes into `develop`.
  - Merge the `develop` into `main` branch. Ensure all test cases pass.
3. **Create Tag**: Create a Git tag from `main` following SemVer (e.g., `git tag v1.2.3`).
4. **Run Tests**: Ensure all test cases pass, including integration, end-to-end tests and the conformance as well?? NEED REVIEW!
5. **Trigger Release**: Pushing the tag triggers the CI pipeline.
6. **Publish Coverage**: Publish test coverage reports.
7. **Continue Development**: Meanwhile the development can be continued in the `develop` branch.
8. **Create Discussion**: Open a discussion thread(automatically on release?)(probably as Github Issue?) for the release to gather feedback.

### Automation Details

- Releases are automated using [GoReleaser](https://goreleaser.com/) triggered by tags matching `v*` in [.github/workflows/release.yml](.github/workflows/release.yml).
- GoReleaser handles building binaries, Docker images, checksums, and changelogs as defined in [.goreleaser.yaml](.goreleaser.yaml).
- For Helm charts, ensure the version in [helmchart/Chart.yaml](helmchart/Chart.yaml) is updated to match the release version (currently hardcoded at 0.1.0; automate this in future).

## Commit Style

To inform breaking changes and facilitate automated versioning/changelogs, use [Conventional Commits](https://conventionalcommits.org/):

- Format: `type(scope): description`
- Common types: `feat` (new feature), `fix` (bug fix), `docs` (documentation), `refactor` (code change), `test` (testing), `chore` (maintenance).
- Scopes: Optional, e.g., `api`, `cli`, `helm`.
- For breaking changes: Include `BREAKING CHANGE:` in the commit footer, e.g.:
  ```
  feat(api): change user endpoint

  BREAKING CHANGE: The user endpoint now requires authentication.
  ```
- This allows automation tools to generate accurate changelogs and determine version bumps. Enforce with tools like `commitlint`.

## Immutable Releases

Immutable releases mean once a version is released, it cannot be modified. This ensures stability and reproducibility but complicates hotfixes.

### Pros
- Predictable deployments: No risk of a release changing after deployment.
- Reproducibility: Always deploy the exact same artifacts.
- Security: Prevents tampering with released versions.

### Cons
- Hotfixes require new releases: Cannot patch an existing release in-place.
- Version proliferation: May lead to many patch versions.

### Proposed Strategy
- Treat releases as immutable.
- For fixes: Create patch releases (increment PATCH) with the fix.
- Avoid in-place updates; instead, deploy new versions.
- For emergencies: Use pre-release labels (e.g., `1.2.4-rc.1`) for quick fixes before stable release.
- Exceptions: If immutability must be violated (e.g., critical security flaw), document the change and communicate transparently.

## Deliverables

Each release produces the following artifacts:

1. **Release Notes**: Auto-generated summary of changes, linked to commits. Stored in GitHub Releases.
2. **Binary Files and Checksums**: Platform-specific binaries (e.g., tar.gz for Linux amd64/arm64) with SHA256 checksums for verification. Available in GitHub Releases.
3. **Container Images**: Multi-platform Docker images pushed to GHCR (e.g., `ghcr.io/margo/workload-fleet-management-client:v1.2.3`). Verify with `docker pull` and checksums.
4. **Changelog**: Markdown changelog generated by GoReleaser, excluding documentation-only commits. Included in release notes.

## Tools and Automation

- **GoReleaser**: Core tool for building and releasing. Configured in [.goreleaser.yaml](.goreleaser.yaml) to build binaries, images, and generate metadata.
- **CI Pipeline for Release**: Triggered on tag push via [.github/workflows/release.yml](.github/workflows/release.yml).
- **Gaps Addressed**:
  - Automate Helm chart versioning: Update [helmchart/Chart.yaml](helmchart/Chart.yaml) and [helmchart/values.yaml](helmchart/values.yaml) to use semantic tags instead of `:latest`. Implement via CI scripts or GoReleaser hooks (e.g., sed commands to replace versions).
  - Image tagging: Use versioned tags (e.g., `:v1.2.3`) for production; keep `:latest` for development builds.
- **CommitLint**: To ensure that conventional-commits are followed.
- **gosec**: Security focused linter for Go.
- **Go test**: For test coverage and report it via Codecov.
- **Dependabot**: Automated dep updates to avoid vulnerable.

## Backwards Compatibility

- **Policy**: Maintain backwards compatibility within major versions. Breaking changes require a major version bump.
- **Deprecation**: Introduce deprecation warnings in minor versions for features to be removed in the next major version. Timeline: Warn in version N.x, remove in (N+1).0.
- **Migration Guides**: Provide guides for breaking changes, including upgrade paths and timelines. Example: For API changes, include code samples and rollback instructions.
- **Testing**: Include backwards compatibility tests in CI to prevent regressions.

## Branch Naming

Use consistent branch naming to support a Git Flow-like workflow:

- `main`: Stable releases, production-ready code.
- `develop`: Active development branch for features and fixes.
- `release/vX.Y`: Release preparation branches (e.g., `release/v1.2` for stabilizing 1.2.0). ---- REVIEW: an overkill?
- `feature/`: Feature branches (e.g., `feature/new-api`).
- `hotfix/`: Hotfix branches for urgent patches (e.g., `hotfix/security-patch`).
- `chore/`: Maintenance stuff (e.g., `chore/updated-deps`).
- `refactor/`: Remodeling the source code (e.g., `refactor/api-clients`).

Merge features to `develop`, then to `release/` for stabilization, and finally to `main` with tags. Use PRs for merges, require reviews, and resolve conflicts promptly. Long-lived branches should be rebased regularly.

## Testing and Verification

- **Pre-release**: Run full test suite, including unit (e.g., Go tests in pkg/), integration, and end-to-end tests. Ensure coverage >80%.
- **Post-release**: Verify deployments in staging, check logs, and monitor for issues. Run smoke tests on production-like environments.
- **Rollback**: If needed, rollback to previous version by redeploying the prior tag (e.g., `helm rollback` or redeploy previous image).

## Communication

- Announce releases via communication channels.

## Roles and Responsibilities

- **Maintainers**: Handle tagging, merging to main, and final release.
- **Developers**: Ensure commits follow conventional style, implement features/fixes.
- **QA Team**: Run tests, verify deliverables, perform post-release checks.
- **DevOps/Release Manager**: Trigger CI, update Helm charts, monitor deployments.
- **Community**: Provide feedback, report issues post-release.

## Security and Compliance

- Run security scans (e.g., vulnerability checks on dependencies) as part of CI.
- Include SBOM (Software Bill of Materials) in releases for transparency.
- Ensure licenses are compliant.

## Release Checklist

- [ ] Code freeze announced
- [ ] All changes merged to `develop`
- [ ] Tests pass
- [ ] Merge `develop` into `main`
- [ ] Test thoroughly; handle hotfixes via hotfix branches from `main` 
- [ ] Tag created (e.g., `v1.2.3`) from `main` branch
- [ ] Release triggered and artifacts generated
- [ ] Helm chart version updated
- [ ] Images tagged correctly
- [ ] Release notes generated
- [ ] Post-release verification completed
- [ ] Github discussion opened against the Release for any support
- [ ] Communication sent