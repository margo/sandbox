# Contributing to Margo Sandbox

Thank you for contributing! This guide covers development workflow, commit standards, and code quality expectations.

## Getting Started

1. **Fork & Clone**: Fork the repository and clone locally
2. **Branch**: Create a feature branch from `development` (see Branch Naming below)
3. **Setup**: Install dependencies and run tests locally
4. **Code**: Make your changes following the style guide
5. **Test**: Ensure all tests pass and coverage is maintained
6. **Commit**: Use conventional commits (see below)
7. **Push**: Push to your fork
8. **PR**: Create a pull request against `development`

---

## Commit Style

All commits must follow [Conventional Commits](https://conventionalcommits.org/) format:

```
type(scope): description

[optional body]

[optional footer: BREAKING CHANGE: ...]
```

### Commit Types Sample

| Type | Purpose | Affects Version |
|------|---------|-----------------|
| `feat` | New feature or capability | **MINOR** or **MAJOR** bump depending upon whether there is a breaking change |
| `fix` | Bug fix or issue resolution | **PATCH** bump |
| `docs` | Documentation only | No version bump |
| `test` | Test additions/improvements | No version bump |
| `refactor` | Code restructuring (no feature/fix) | No version bump |
| `chore` | Maintenance, deps, tooling | No version bump* |

*`chore(deps)` (dependency updates) = **PATCH** bump

### Rules

- **Scope** (optional): e.g., `cli`, `device-agent`, `helm`, `shared-lib`
- **Description**: lowercase
  - ✅ Good: `feat(cli): add config validation`
  - ❌ Bad: `Added config validation to CLI`
- **BREAKING CHANGE**: Include in footer for API/config or other incompatibilities; triggers **MAJOR** version bump
  ```
  feat(api): change user endpoint structure

  BREAKING CHANGE: The endpoint now returns a different JSON structure.
  See migration guide in docs/API-MIGRATION.md
  ```

### Example Commits

```
feat(cli): add support for config validation
fix(device-agent): resolve connection timeout
chore(deps): update Go to 1.21
feat(wfm): add metrics collection (BREAKING CHANGE: API response structure changed)
docs: update installation guide
```

### Enforcement

- **CommitLint** validates all commits in CI
- Non-conformant commits block PR merge
- Use `git commit --amend` to fix messages before pushing

For details on versioning implications, see [docs/release.md](docs/release.md#versioning).

---

## Branch Naming

Create branches from the appropriate base:

| Branch Type | Pattern | Base | Purpose |
|-------------|---------|------|---------|
| Feature | `feature/short-description` | `development` | New feature (e.g., `feature/device-telemetry`) |
| Bug fix | `fix/short-description` | `development` | Regular bug fixes (e.g., `fix/connection-leak`) |
| Hotfix | `hotfix/short-description` | `main` | Critical production bugs only |
| Maintenance | `chore/short-description` | `development` | Deps, tooling, CI (e.g., `chore/update-go`) |
| Refactoring | `refactor/short-description` | `development` | Code restructuring (e.g., `refactor/api-client`) |
| Documentation | `docs/short-description` | `development` | Doc-only updates |

### Workflow

1. **Create branch**:
   ```bash
   git checkout develop
   git pull origin develop
   git checkout -b feature/my-feature
   ```

2. **Work locally**:
   - Commit regularly with conventional format
   - Push to your fork: `git push origin feature/my-feature`
   - Keep branch up-to-date: `git rebase origin/develop`

3. **Open PR**:
   - Describe what you changed and why
   - Link related issues (e.g., "Fixes #123")
   - Request review from CODEOWNERS

4. **Merge**:
   - Get approval from at least 1 maintainer
   - Squash & merge to base branch (CI enforces this)
   - Delete your branch after merge

---

## Testing & Coverage

### Before You Submit a PR

1. **Run tests locally**:
   ```bash
   go test ./...
   go test -coverprofile=coverage.out ./...
   go tool cover -html=coverage.out
   ```

2. **Ensure coverage**:
   - Minimum **80%** repo-wide coverage required
   - New code should maintain/improve coverage
   - Check [Codecov.io](https://codecov.io) after pushing

3. **Run security scan**:
   ```bash
   go install github.com/securego/gosec/v2/cmd/gosec@latest
   gosec ./...
   ```

4. **Check for lint issues**:
   - Follow Go conventions (gofmt, golint)
   - Tests must pass in CI before merge

### Test Guidelines

- Write tests for new features and bug fixes
- Include both happy path and error cases
- Aim for clear, descriptive test names
- Use table-driven tests for multiple scenarios

---

## Code Style

### Go Standards

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for automatic formatting
- Package names in lowercase, no underscores
- Exported names should be descriptive

### Documentation

- Document public functions and types
- Include examples in package-level comments
- Update README.md if user-facing features change

---

## Dependencies

### Adding New Packages

1. Add to `go.mod` via `go get`
2. **Check license**: MIT, Apache 2.0, BSD, GPL (with approval), or OSI-approved only
   ```bash
   go mod graph | grep import-path
   ```
3. Document why in your commit

### Updating Dependencies

- Dependabot creates PRs automatically
- Critical vulnerabilities are prioritized
- Review and test before merging updates

---

## Pull Request Process

### PR Checklist

- [ ] Branch created from correct base (`development` or `main` for hotfix)
- [ ] Commits follow conventional format
- [ ] All tests pass locally
- [ ] Coverage maintained or improved
- [ ] No high-severity security issues (GoSec passes)
- [ ] PR description is clear and links related issues
- [ ] Requested reviews from CODEOWNERS

### PR Description Template

```markdown
## Description
[Brief summary of changes]

## Motivation
[Why this change? What problem does it solve?]

## Testing
[How did you test this? What scenarios?]

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] No breaking changes (or documented in footer)
- [ ] Conventional commits used
```

### Responding to Reviews

- Address feedback promptly
- Push additional commits (CI will squash on merge)
- Request re-review when ready
- Ask questions if feedback is unclear

---

## Reporting Issues

1. **Search existing issues** before creating new ones
2. **Include details**:
   - Steps to reproduce
   - Expected vs actual behavior
   - OS, Go version, environment
   - Error logs/stack traces
3. **Use labels**: `bug`, `enhancement`, `question`, etc.
4. **Link related PRs** if applicable

---

## Release Process

For details on versioning, scheduling, and release procedures, see [docs/release.md](docs/release.md).

Key points for contributors:
- You **don't need to release**; maintainers handle it
- Your commits' `type` (feat, fix, docs) automatically determine version bumps
- Use `BREAKING CHANGE:` footer if you make incompatible changes
- Review release notes after your PR merges to ensure they reflect your contribution

---

## Questions?

- **Bug reports**: [GitHub Issues](https://github.com/margo/sandbox/issues)
- **Feature requests**: Via [SUPs](https://github.com/margo)
- **Help**: [Discourse Channel](https://discourse.margo.org/)

Thank you for contributing!
