# Contributing

Thank you for your interest in contributing to the `digital.vasic.messaging` module. This document outlines the process and standards for contributions.

## Prerequisites

- Go 1.24.0 or later
- Git with SSH access
- Familiarity with Go modules, interfaces, and concurrency primitives

## Getting Started

1. Clone the repository using SSH:

```bash
git clone <ssh-url>
cd Messaging
```

2. Install dependencies:

```bash
go mod tidy
```

3. Run the test suite:

```bash
go test ./... -count=1 -race
```

All tests must pass before submitting changes.

## Development Workflow

### Branch Naming

Use the following prefixes:

- `feat/` -- New features (e.g., `feat/nats-adapter`)
- `fix/` -- Bug fixes (e.g., `fix/retry-backoff-overflow`)
- `chore/` -- Maintenance tasks (e.g., `chore/update-uuid-dep`)
- `docs/` -- Documentation only (e.g., `docs/add-nats-examples`)
- `refactor/` -- Code restructuring (e.g., `refactor/extract-subscription-interface`)
- `test/` -- Test additions or improvements (e.g., `test/batch-consumer-edge-cases`)

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>
```

Examples:

```
feat(broker): add NATS adapter with JetStream support
fix(consumer): prevent goroutine leak in BatchConsumer.Stop
test(producer): add table-driven tests for GzipCompressor
docs(api): document RetryPolicy.Delay formula
refactor(kafka): extract subscription lifecycle into helper
```

### Code Quality Checks

Before committing, run:

```bash
gofmt -w .
go vet ./...
go test ./... -count=1 -race
```

All three must pass cleanly.

## Code Standards

### Go Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go) and standard `gofmt` formatting.
- Use `goimports` for import grouping: stdlib, third-party, internal (separated by blank lines).
- Line length should not exceed 100 characters where practical.

### Naming

- **Private**: `camelCase` (e.g., `publishFn`, `subCounter`)
- **Exported**: `PascalCase` (e.g., `NewProducer`, `BatchConsumer`)
- **Constants**: `UPPER_SNAKE_CASE` for environment-style, `PascalCase` for typed constants
- **Acronyms**: All caps (e.g., `ID`, `URL`, `TLS`, `SASL`, `DLQ`)
- **Receivers**: 1-2 letters reflecting the type (e.g., `b` for broker, `c` for consumer, `p` for producer, `s` for serializer)

### Error Handling

- Always check errors. Never discard them silently (except in deferred Close calls where the error is not actionable).
- Wrap errors with context: `fmt.Errorf("failed to subscribe to %s: %w", topic, err)`.
- Use `broker.NewBrokerError` for structured errors in adapter code.
- Use sentinel errors (`broker.ErrNotConnected`, etc.) for conditions that callers need to match with `errors.Is`.

### Interfaces

- Keep interfaces small and focused. One method is ideal; more than five is a smell.
- Accept interfaces, return concrete types.
- Define interfaces in the package that uses them, not the package that implements them (except for `broker.MessageBroker` and `broker.Subscription` which are the foundational contracts).

### Concurrency

- Always pass `context.Context` as the first parameter.
- Use `sync/atomic` for simple state flags and counters.
- Use `sync.RWMutex` for protecting maps and slices accessed by multiple goroutines.
- Use buffered channels for work queues; never use unbuffered channels for decoupled components.
- Ensure all goroutines are stoppable via context cancellation or stop channels.

### Testing

- Use table-driven tests with descriptive subtest names.
- Use `github.com/stretchr/testify/assert` and `require` for assertions.
- Test naming: `Test<Type>_<Method>_<Scenario>` (e.g., `TestRetryPolicy_Delay_ExponentialBackoff`).
- No external infrastructure in tests. Use `InMemoryBroker` and injectable functions.
- Run with `-race` to detect data races.

## Adding a New Broker Adapter

1. Create `pkg/<brokername>/<brokername>.go`.
2. Define a `Config` struct with broker-specific settings and a `Validate() error` method.
3. Provide a `DefaultConfig() *Config` factory function.
4. Implement `Producer` and `Consumer` types with `Connect`, `Close`, `IsConnected`, `Publish`/`Subscribe`, `Unsubscribe`, and `Config()` methods.
5. Use the injectable function pattern (`SetPublishFunc`) instead of importing broker client libraries.
6. Add a `BrokerType` constant in `pkg/broker/broker.go` and update `IsValid()`.
7. Create `pkg/<brokername>/<brokername>_test.go` with comprehensive tests.
8. Update documentation: `README.md`, `docs/USER_GUIDE.md`, `docs/API_REFERENCE.md`, `docs/ARCHITECTURE.md`.

## Adding a New Serializer or Compressor

1. Implement the `Serializer` or `Compressor` interface in `pkg/producer/producer.go`.
2. If the implementation requires external dependencies, use the injectable function pattern (like `SnappyCompressor`).
3. Add tests.
4. Update `docs/API_REFERENCE.md` and `docs/USER_GUIDE.md`.

## Pull Request Process

1. Create a feature branch from `main`.
2. Make your changes following all standards above.
3. Ensure all tests pass: `go test ./... -count=1 -race`.
4. Ensure code is formatted: `gofmt -d .` should produce no output.
5. Ensure no vet warnings: `go vet ./...`.
6. Update documentation if your changes affect the public API.
7. Update `docs/CHANGELOG.md` with your changes under an "Unreleased" section.
8. Submit a pull request with a clear description of the changes and motivation.

## Dependency Policy

This module intentionally has minimal dependencies:

- `github.com/google/uuid` -- UUID generation
- `github.com/stretchr/testify` -- Test assertions (test-only)

Do not add broker client libraries (kafka-go, amqp091-go, nats.go, etc.) as dependencies. Use the injectable function pattern instead. This keeps the module lightweight and avoids version conflicts for consumers.

Any new dependency must be justified and approved before merging.

## Questions

If you have questions about contributing, open an issue or reach out to the maintainers.
