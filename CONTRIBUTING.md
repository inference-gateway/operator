# Contributing to Inference Gateway Operator

Thank you for your interest in contributing to the Inference Gateway Operator project! This document provides detailed guidelines to help you contribute effectively.

## Table of Contents

- [Contributing to Inference Gateway Operator](#contributing-to-inference-gateway-operator)
  - [Table of Contents](#table-of-contents)
  - [Code of Conduct](#code-of-conduct)
  - [Getting Started](#getting-started)
    - [Development Environment](#development-environment)
    - [Setting Up Your Environment](#setting-up-your-environment)
    - [Development Workflow](#development-workflow)
  - [Making Contributions](#making-contributions)
    - [Finding Issues to Work On](#finding-issues-to-work-on)
    - [Creating Issues](#creating-issues)
    - [Submitting Pull Requests](#submitting-pull-requests)
    - [Code Review Process](#code-review-process)
  - [Development Guidelines](#development-guidelines)
    - [Code Style](#code-style)
    - [Testing Requirements](#testing-requirements)
    - [Documentation](#documentation)
  - [Project Structure](#project-structure)
  - [Adding Features](#adding-features)
    - [API Changes](#api-changes)
    - [Controller Changes](#controller-changes)
  - [Release Process](#release-process)
  - [Getting Help](#getting-help)

## Code of Conduct

Please be respectful and constructive in all interactions related to this project. We expect all contributors to foster an inclusive and welcoming environment for everyone.

## Getting Started

### Development Environment

This project is developed using a containerized development environment with the following tools pre-installed:

- **Git** (latest version) - Built from source and available on the `PATH`
- **Go** and common Go utilities - Pre-installed and available on the `PATH`, with the Go language extension for Go development
- **Node.js, npm, and ESLint** - Pre-installed and available on the `PATH` for JavaScript development
- **Docker CLI** - Pre-installed and available on the `PATH` for running and managing containers using a dedicated Docker daemon within the dev container

### Setting Up Your Environment

1. **Fork the repository** and clone your fork:

   ```sh
   git clone https://github.com/YOUR-USERNAME/operator.git
   cd operator
   ```

2. **Set up your development environment**:

   - Option A: Use the provided dev container configuration (recommended)
     ```sh
     # If using VS Code with the Remote - Containers extension
     # Simply open the folder in VS Code and click "Reopen in Container"
     # when prompted
     ```
   - Option B: Set up your local environment with the required dependencies
     ```sh
     # Ensure you have Go 1.23+, Docker 17.03+, and kubectl 1.11.3+ installed
     ```

3. **Install development dependencies**:

   ```sh
   go mod download
   ```

4. **Create a branch** for your changes:
   ```sh
   git checkout -b feature/your-feature-name
   ```

### Development Workflow

1. **Make your changes** following the code style and guidelines
2. **Run tests** to ensure your changes don't break existing functionality:
   ```sh
   task test
   # For specific test coverage
   task test-coverage
   ```
3. **Run end-to-end tests** to verify integration functionality:
   ```sh
   task e2e-test
   ```
4. **Lint your code** to ensure consistency with the codebase:
   ```sh
   task lint
   ```
5. **Build and verify** the project locally:
   ```sh
   task build
   ```

## Making Contributions

### Finding Issues to Work On

- Check the [Issues](https://github.com/inference-gateway/operator/issues) tab for open issues labeled as `good-first-issue` or `help-wanted`.
- If you're interested in implementing a new feature, please discuss it first by creating an issue.

### Creating Issues

When creating a new issue, please include:

- For feature requests: Clear use cases and rationale for the feature
- For bug reports: Steps to reproduce, expected behavior, actual behavior
- If possible, include information about your environment (Kubernetes version, etc.)

### Submitting Pull Requests

1. **Commit your changes** with a clear, descriptive commit message:

   ```sh
   git commit -m "Add feature X that does Y"
   ```

   Commit messages should follow these guidelines:

   - Start with a verb in imperative mood (Add, Fix, Update, etc.)
   - Keep the first line under 72 characters
   - Reference issue numbers if applicable (e.g., "Fixes #123")

2. **Push to your fork**:

   ```sh
   git push origin feature/your-feature-name
   ```

3. **Create a Pull Request** against the main repository's main branch

   - Provide a clear description of the changes
   - Link to any relevant issues
   - Fill out the PR template if one exists

4. **Respond to feedback** from maintainers and other reviewers

### Code Review Process

- All PRs require at least one approval from a maintainer
- Automated checks must pass (tests, linting, etc.)
- Be responsive to feedback and make requested changes

## Development Guidelines

### Code Style

- **Go Code**:
  - Follow standard Go coding conventions as outlined in [Effective Go](https://golang.org/doc/effective_go)
  - Use [gofmt](https://golang.org/cmd/gofmt/) to format your code
  - Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) guidelines
- **General Guidelines**:
  - Use meaningful variable and function names
  - Write clear comments for complex logic
  - Keep functions focused and relatively small
  - Group related functionality

### Testing Requirements

- All new code should include appropriate unit tests
- End-to-end tests should be added for significant features
- Test coverage should ideally be maintained or improved with each PR
- To run tests:

  ```sh
  # Run unit tests
  task test

  # Run e2e tests
  task e2e-test
  ```

### Documentation

- Update documentation when changing behavior
- Document public API changes
- Include code examples for complex features
- Update the README.md if necessary

## Project Structure

The project follows the standard Kubernetes operator pattern:

- `api/` - API definitions and generated client code
- `cmd/` - Entry point for the operator
- `config/` - Kubernetes manifests and configuration
- `internal/` - Internal implementation code
- `test/` - Test code and utilities

## Adding Features

### API Changes

1. **For new API fields**:

   - Update the CRD in `api/v1alpha1/gateway_types.go`
   - Add appropriate validation, defaults, and descriptions
   - Run `task generate` to update generated code
   - Update sample CRDs in `config/samples/`

2. **For new API types**:
   - Create new type files in the appropriate version directory
   - Register types in `api/v1alpha1/groupversion_info.go`
   - Run `task generate` to update generated code

### Controller Changes

1. **For new controller functionality**:

   - Update the relevant controller in `internal/controller/`
   - Add test coverage in the corresponding `_test.go` file
   - Update reconciliation logic as needed
   - Consider edge cases and error handling

2. **For new controllers**:
   - Create a new controller file
   - Implement the reconciliation interface
   - Register the controller in the manager
   - Add appropriate RBAC permissions

## Release Process

1. **Versioning**

   - We follow [Semantic Versioning](https://semver.org/)
   - Tag releases using Git tags

2. **Release Process**
   - Update version numbers in code
   - Update CHANGELOG.md
   - Create a release tag
   - Build and publish container images

## Getting Help

- For questions about contributing, create an issue with the label `question`
- Join our community channels (if applicable)
- Reach out to project maintainers directly

We appreciate your contributions to making the Inference Gateway Operator better!
