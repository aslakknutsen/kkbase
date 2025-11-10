# Contributing to kkbase

Thank you for your interest in contributing to kkbase!

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally
3. **Set up your development environment** - see [Local Development Guide](docs/getting-started/local-development.md)

## Development Workflow

1. Create a feature branch: `git checkout -b feature/my-feature`
2. Make your changes
3. Run tests: `make test`
4. Run linting: `golangci-lint run`
5. Commit your changes with descriptive messages
6. Push to your fork
7. Open a Pull Request

## Code Style

- Follow Go conventions and best practices
- Use `gofmt` for formatting
- Write tests for new features
- Update documentation as needed

## Pull Request Guidelines

- Keep PRs focused on a single feature or fix
- Include tests and documentation
- Ensure all tests pass
- Reference related issues

## Commit Message Format

Follow conventional commits:

```
feat: add new MCP tool for blast zone analysis
fix: correct pod relationship extraction
docs: update installation guide
test: add integration tests for agent sessions
```

## Questions?

- Open a [GitHub Issue](https://github.com/aslakknutsen/kkbase/issues)
- Start a [GitHub Discussion](https://github.com/aslakknutsen/kkbase/discussions)

## Resources

- [Development Guide](docs/development/)
- [Architecture Documentation](docs/ARCHITECTURE.md)
- [Building Guide](docs/development/building.md)
- [Testing Guide](docs/development/testing.md)

