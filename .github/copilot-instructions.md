# Custom Instructions for Copilot

Today is June 20, 2025.

- Always use context7 to check for the latest updates, features, or best practices of a library relevant to the task at hand.
- Always prefer Table-Driven Testing: When writing tests.
- Always use Early Returns: Favor early returns to simplify logic and avoid deep nesting with if-else structures.
- Always prefer switch statements over if-else chains: Use switch statements for cleaner and more readable code when checking multiple conditions.
- Always run `task lint` before committing code to ensure it adheres to the project's linting rules.
- Always run `task generate` to ensure all types and code are up-to-date after making changes to schemas or API definitions.
- Always run `task manifests` to ensure Kubernetes manifests are up-to-date.
- Always run `task fmt` to ensure code formatting is consistent.
- Always run `task build` to verify compilation after making changes.
- Always run `task test` before committing code to ensure all tests pass.
- Always search for the simplest solution first before considering more complex alternatives.
- Always prefer type safety over dynamic typing: Use strong typing and interfaces to ensure type safety and reduce runtime errors.
- Always use lowercase log messages for consistency and readability.
- When possible code to an interface so it's easier to mock in tests.
- When writing tests, each test case should have it's own isolated mock server mock dependecies so it's easier to understand and maintain.

## Development Workflow

1. Run `task lint` and `lint-fix` to ensure code quality.
2. Run `task generate` If added new Schemas, for generating types and code.
3. Run `task manifests` to ensure Kubernetes manifests are up-to-date.
4. Run `task fmt` to ensure code formatting is consistent.
5. Run `task build` to verify successful compilation.
6. Run `task test` to ensure all tests pass.
7. Run `task test:e2e` to run all end-to-end tests if applicable.
8. Run `task test:e2e:focus` to run focused end-to-end test if applicable.

## Available Tools and MCPs

- context7 - Helps by finding the latest updates, features, or best practices of a library relevant to the task at hand.

## Useful Links

- **[Operator SDK](https://sdk.operatorframework.io/docs/)** - Official documentation for Operator SDK

## Related Repositories

### Core Inference Gateway

- **[Main Repository](https://github.com/inference-gateway)** - The main inference gateway org.
- **[Documentation](https://github.com/inference-gateway/docs)** - Official documentation and guides
- **[UI](https://github.com/inference-gateway/ui)** - Web interface for the inference gateway

### SDKs & Client Libraries

- **[Go SDK](https://github.com/inference-gateway/go-sdk)** - Go client library
- **[Rust SDK](https://github.com/inference-gateway/rust-sdk)** - Rust client library
- **[TypeScript SDK](https://github.com/inference-gateway/typescript-sdk)** - TypeScript/JavaScript client library
- **[Python SDK](https://github.com/inference-gateway/python-sdk)** - Python client library

### A2A (Agent-to-Agent) Ecosystem

- **[Awesome A2A](https://github.com/inference-gateway/awesome-a2a)** - Curated list of A2A-compatible agents
- **[Google Calendar Agent](https://github.com/inference-gateway/google-calendar-agent)** - Agent for Google Calendar integration

### Internal Tools

- **[Internal Tools](https://github.com/inference-gateway/tools)** - Collection of internal tools and utilities
