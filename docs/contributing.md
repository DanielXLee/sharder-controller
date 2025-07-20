# Contributing Guide

## Welcome

Thank you for your interest in contributing to the Kubernetes Shard Controller! This guide will help you get started with contributing to the project.

## Code of Conduct

This project adheres to the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md). By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Go 1.24 or later
- Docker (for building images)
- kubectl and access to a Kubernetes cluster
- Git

### Development Setup

1. **Fork the repository** on GitHub
2. **Clone your fork**:
   ```bash
   git clone https://github.com/YOUR_USERNAME/shard-controller.git
   cd shard-controller
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/k8s-shard-controller/shard-controller.git
   ```
4. **Install dependencies**:
   ```bash
   go mod download
   ```
5. **Verify setup**:
   ```bash
   make test
   ```

## How to Contribute

### Reporting Issues

- Use GitHub Issues to report bugs or request features
- Search existing issues before creating new ones
- Provide detailed information including:
  - Steps to reproduce
  - Expected vs actual behavior
  - Environment details (Kubernetes version, etc.)
  - Logs and error messages

### Suggesting Features

- Open a GitHub Issue with the "enhancement" label
- Describe the use case and expected behavior
- Discuss the proposal with maintainers before implementation

### Contributing Code

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make your changes**:
   - Follow the coding standards
   - Add tests for new functionality
   - Update documentation as needed

3. **Test your changes**:
   ```bash
   make test
   make test-integration
   make lint
   ```

4. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat: add new feature description"
   ```

5. **Push to your fork**:
   ```bash
   git push origin feature/my-new-feature
   ```

6. **Create a Pull Request** on GitHub

## Development Guidelines

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `make lint` to check for issues
- Add comments for exported functions and types

### Commit Messages

Use conventional commit format:
```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Build/tooling changes

Examples:
```
feat(controller): add support for custom metrics
fix(worker): resolve memory leak in heartbeat manager
docs(api): update ShardConfig specification
```

### Testing

- Write unit tests for new functionality
- Add integration tests for complex features
- Ensure all tests pass before submitting PR
- Aim for good test coverage

### Documentation

- Update relevant documentation for changes
- Add examples for new features
- Keep API documentation current
- Update troubleshooting guide if needed

## Pull Request Process

### Before Submitting

- [ ] Tests pass (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated
- [ ] Commit messages follow convention

### PR Requirements

1. **Clear Description**: Explain what the PR does and why
2. **Issue Reference**: Link to related issues
3. **Testing**: Describe how changes were tested
4. **Breaking Changes**: Clearly mark any breaking changes
5. **Documentation**: Update docs for user-facing changes

### Review Process

1. **Automated Checks**: All CI checks must pass
2. **Code Review**: At least one maintainer review required
3. **Testing**: Reviewers may test changes locally
4. **Approval**: Maintainer approval required for merge

### After Submission

- Respond to review feedback promptly
- Make requested changes in additional commits
- Keep PR up to date with main branch
- Be patient - reviews take time

## Development Workflow

### Building

```bash
# Build all components
make build

# Build specific components
make manager
make worker

# Build Docker images
make docker-build
```

### Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run with coverage
make coverage

# Run specific tests
go test -v ./pkg/controllers/...
```

### Local Development

```bash
# Run locally against cluster
make run

# Run with debug logging
./bin/manager --log-level debug

# Run quick demo
./scripts/local-demo.sh
```

## Project Structure

```
shard-controller/
├── cmd/                    # Main applications
├── pkg/                    # Library code
├── manifests/              # Kubernetes manifests
├── docs/                   # Documentation
├── examples/               # Example configurations
├── test/                   # Test files
├── scripts/                # Build and utility scripts
└── hack/                   # Development scripts
```

## Areas for Contribution

### Good First Issues

Look for issues labeled `good first issue`:
- Documentation improvements
- Small bug fixes
- Test additions
- Example configurations

### Help Wanted

Issues labeled `help wanted`:
- Feature implementations
- Performance improvements
- Integration work
- Advanced testing

### Priority Areas

- Performance optimization
- Additional load balancing strategies
- Enhanced monitoring and metrics
- Multi-cluster support
- Security enhancements

## Community

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and general discussion
- **Slack**: Real-time chat (link in README)

### Getting Help

- Check existing documentation first
- Search GitHub issues and discussions
- Ask questions in GitHub Discussions
- Join community Slack for real-time help

### Community Guidelines

- Be respectful and inclusive
- Help others when you can
- Share knowledge and experiences
- Follow the code of conduct

## Release Process

### Versioning

We follow semantic versioning (SemVer):
- **Major**: Breaking changes
- **Minor**: New features (backward compatible)
- **Patch**: Bug fixes (backward compatible)

### Release Cycle

- Regular minor releases every 2-3 months
- Patch releases as needed for critical fixes
- Pre-releases for testing major changes

## Recognition

Contributors are recognized in:
- Release notes
- Contributors file
- Community acknowledgments

## Questions?

If you have questions about contributing:

1. Check this guide and other documentation
2. Search existing GitHub issues and discussions
3. Ask in GitHub Discussions
4. Reach out on community Slack

Thank you for contributing to the Kubernetes Shard Controller! 🎉