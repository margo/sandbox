# Navigating the Codebase

A comprehensive guide to understanding and working with this Go project.

## Table of Contents

- [Project Structure Overview](#project-structure-overview)
- [Getting Started](#getting-started)
- [Understanding the Architecture](#understanding-the-architecture)
- [Key Directories and Files](#key-directories-and-files)
- [Code Organization Patterns](#code-organization-patterns)
- [Development Workflow](#development-workflow)
- [Debugging and Troubleshooting](#debugging-and-troubleshooting)
- [Contributing Guidelines](#contributing-guidelines)

## Project Structure Overview

### Standard Go Project Layout

```
├── cmd/                   # Main applications
│   ├── server/            # Server application entry point
│   └── cli/               # CLI tool entry point
├── internal/              # Private application code
│   ├── api/               # API layer (handlers, middleware)
│   ├── service/           # Business logic layer
│   ├── repository/        # Data access layer
│   └── config/            # Configuration management
├── pkg/                   # Public library code
│   ├── client/            # Client libraries
│   ├── models/            # Shared data models
│   └── utils/             # Utility functions
├── api/                   # API definitions (OpenAPI/Swagger, protobuf)
├── web/                   # Web assets (if applicable)
├── scripts/               # Build and deployment scripts
├── deployments/           # Deployment configurations
├── test/                  # Integration and e2e tests
├── docs/                  # Documentation
├── examples/              # Example code and usage
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── Makefile              # Build automation
└── README.md             # Project overview
```

## Getting Started

### Prerequisites

1. **Go Version**: Check `go.mod` for the required Go version
2. **Dependencies**: Review external dependencies in `go.mod`
3. **Tools**: Install required development tools

```bash
# Install development dependencies
make install-tools

# Or manually install common tools
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Initial Setup

```bash
# Clone and setup
git clone <repository-url>
cd <project-name>

# Download dependencies
go mod download

# Verify setup
make test
make build
```

### Running the Application

```bash
# Development mode
make run

# Or directly
go run cmd/server/main.go

# With configuration
go run cmd/server/main.go -config=configs/dev.yaml
```

## Understanding the Architecture

### Architectural Patterns

This project follows these key patterns:

1. **Clean Architecture**: Separation of concerns across layers
2. **Dependency Injection**: Using interfaces for loose coupling
3. **Repository Pattern**: Data access abstraction
4. **Service Layer**: Business logic encapsulation

### Layer Responsibilities

```
┌─────────────────┐
│   Presentation  │ ← cmd/, internal/api/
│     Layer       │   (HTTP handlers, CLI commands)
├─────────────────┤
│   Business      │ ← internal/service/
│     Layer       │   (Core business logic)
├─────────────────┤
│   Data Access   │ ← internal/repository/
│     Layer       │   (Database, external APIs)
├─────────────────┤
│   Infrastructure│ ← pkg/, external dependencies
│     Layer       │   (Utilities, third-party integrations)
└─────────────────┘
```

### Data Flow

```
HTTP Request → Router → Handler → Service → Repository → Database
                 ↓         ↓         ↓          ↓
              Middleware  Validation  Business   Data
              Auth/Logs   Transform   Logic      Access
```

## Key Directories and Files

### `/cmd` - Application Entry Points

- **Purpose**: Main applications for this project
- **Pattern**: Each subdirectory is a separate executable
- **Key Files**:
  - `main.go`: Application entry point
  - `root.go`: CLI root command (if using Cobra)

```go
// Example: cmd/server/main.go
func main() {
    cfg := config.Load()
    app := application.New(cfg)
    app.Run()
}
```

### `/internal` - Private Application Code

- **Purpose**: Code that shouldn't be imported by other projects
- **Key Subdirectories**:

#### `/internal/api`
- HTTP handlers and middleware
- Request/response models
- Route definitions

```go
// Example structure
internal/api/
├── handlers/
│   ├── user.go
│   └── auth.go
├── middleware/
│   ├── auth.go
│   └── logging.go
└── routes.go
```

#### `/internal/service`
- Business logic implementation
- Service interfaces and implementations
- Domain-specific operations

```go
// Example: internal/service/user.go
type UserService interface {
    CreateUser(ctx context.Context, req CreateUserRequest) (*User, error)
    GetUser(ctx context.Context, id string) (*User, error)
}

type userService struct {
    repo UserRepository
    // other dependencies
}
```

#### `/internal/repository`
- Data access layer
- Database operations
- External API integrations

### `/pkg` - Public Library Code

- **Purpose**: Code that can be imported by other projects
- **Examples**: Client libraries, shared models, utilities

### Configuration Management

Look for configuration in these locations:
- `internal/config/` - Configuration structures and loading
- `configs/` - Configuration files
- Environment variables
- Command-line flags

## Code Organization Patterns

### Interface-First Design

```go
// Define interfaces in the package that uses them
type UserService interface {
    CreateUser(ctx context.Context, user *User) error
}

// Implement in separate files/packages
type userService struct {
    repo UserRepository
}

func (s *userService) CreateUser(ctx context.Context, user *User) error {
    // implementation
}
```

### Error Handling

```go
// Custom error types
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

// Error wrapping
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}
```

### Context Usage

```go
// Always pass context as first parameter
func (s *service) ProcessData(ctx context.Context, data []byte) error {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    // Continue processing
}
```

## Development Workflow

### Code Style and Standards

```bash
# Format code
make fmt
# or
gofmt -w .
goimports -w .

# Lint code
make lint
# or
golangci-lint run

# Run tests
make test
# or
go test ./...

# Run tests with coverage
make test-coverage
```

### Testing Strategy

#### Unit Tests
- Located alongside source code (`*_test.go`)
- Test individual functions/methods
- Use table-driven tests when appropriate

```go
func TestUserService_CreateUser(t *testing.T) {
    tests := []struct {
        name    string
        input   *User
        want    error
        wantErr bool
    }{
        // test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

#### Integration Tests
- Located in `/test` directory
- Test component interactions
- May require external dependencies

### Build and Deployment

```bash
# Local build
make build

# Cross-platform builds
make build-all

# Docker build
make docker-build

# Deploy
make deploy
```

## Debugging and Troubleshooting

### Logging

Look for logging configuration in:
- `internal/config/` - Log level and format settings
- Environment variables: `LOG_LEVEL`, `LOG_FORMAT`

```go
// Common logging patterns
log.WithFields(log.Fields{
    "user_id": userID,
    "action":  "create_user",
}).Info("Creating new user")
```

### Common Issues

1. **Import Cycles**: Check for circular dependencies
2. **Missing Dependencies**: Run `go mod tidy`
3. **Version Conflicts**: Check `go.sum` for version mismatches
4. **Configuration**: Verify environment variables and config files

### Debugging Tools

```bash
# Race condition detection
go test -race ./...

# Memory profiling
go test -memprofile=mem.prof
go tool pprof mem.prof

# CPU profiling
go test -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

## Contributing Guidelines

### Before Making Changes

1. **Understand the Issue**: Read related documentation and code
2. **Check Existing Issues**: Avoid duplicate work
3. **Discuss Major Changes**: Open an issue for significant modifications

### Development Process

1. **Create Feature Branch**: `git checkout -b feature/your-feature-name`
2. **Write Tests**: Add tests for new functionality
3. **Follow Code Style**: Use project's formatting and conventions
4. **Update Documentation**: Update relevant docs and comments
5. **Test Thoroughly**: Run all tests and linting
6. **Create Pull Request**: Provide clear description and context

### Code Review Checklist

- [ ] Code follows project conventions
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] No breaking changes (or properly documented)
- [ ] Error handling is appropriate
- [ ] Performance considerations addressed

### Useful Commands

```bash
# Find TODO/FIXME comments
grep -r "TODO\|FIXME" --include="*.go" .

# Find unused code
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...

# Dependency analysis
go mod graph
go mod why <package>

# Generate documentation
godoc -http=:6060
```

## Additional Resources

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Project-specific documentation](./docs/)

---

**Need Help?**
- Check existing issues and documentation
- Ask questions in team chat/forums
- Reach out to maintainers for guidance

**Remember**: Good code is not just working code—it's readable, maintainable, and well-tested code that others can understand and build upon.