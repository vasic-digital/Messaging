# AGENTS.md

Multi-agent coordination guide for the `digital.vasic.messaging` module.

## Module Identity

- **Module path**: `digital.vasic.messaging`
- **Language**: Go 1.24.0
- **Packages**: `pkg/broker`, `pkg/kafka`, `pkg/rabbitmq`, `pkg/consumer`, `pkg/producer`
- **Role**: Generic, reusable message broker abstraction layer with adapter pattern

## Agent Responsibilities

### Broker Agent

- **Scope**: `pkg/broker/` -- core interfaces, message types, error system, in-memory broker
- **Owns**: `MessageBroker` interface, `Message` struct, `Subscription` interface, `Handler` type, `Config`, `BrokerType`, all error types (`BrokerError`, `MultiError`, sentinel errors)
- **Constraints**: This package has zero internal imports. It is the foundation of the module. All other packages depend on it. Changes here ripple everywhere.
- **Testing**: `InMemoryBroker` is the canonical test double. All cross-package tests use it. Never introduce external infrastructure dependencies in broker tests.

### Kafka Agent

- **Scope**: `pkg/kafka/` -- Kafka adapter with Producer, Consumer, and Config
- **Owns**: `kafka.Config`, `kafka.Producer`, `kafka.Consumer`, `PartitionStrategy` enum
- **Constraints**: Must not import any Kafka client library (kafka-go, confluent-kafka-go, etc.). All actual I/O is injectable via `SetPublishFunc`. Config validation is self-contained.
- **Dependencies**: `pkg/broker` only.

### RabbitMQ Agent

- **Scope**: `pkg/rabbitmq/` -- RabbitMQ adapter with Producer, Consumer, and Config
- **Owns**: `rabbitmq.Config`, `rabbitmq.Producer`, `rabbitmq.Consumer`, `ExchangeType` enum
- **Constraints**: Must not import any AMQP client library (amqp091-go, etc.). All actual I/O is injectable via `SetPublishFunc`. Config validation is self-contained. `ConnectionString()` generates AMQP URIs.
- **Dependencies**: `pkg/broker` only.

### Consumer Agent

- **Scope**: `pkg/consumer/` -- higher-level consumption patterns
- **Owns**: `ConsumerGroup`, `RetryPolicy`, `DeadLetterHandler`, `BatchConsumer`, `WithRetry` wrapper
- **Constraints**: Operates on the `broker.MessageBroker` interface. Never references Kafka or RabbitMQ directly. Must remain broker-agnostic.
- **Dependencies**: `pkg/broker` only.

### Producer Agent

- **Scope**: `pkg/producer/` -- higher-level production patterns
- **Owns**: `AsyncProducer`, `SyncProducer`, `Serializer` interface, `Compressor` interface, `JSONSerializer`, `ProtobufSerializer`, `AvroSerializer`, `GzipCompressor`, `SnappyCompressor`, `LZ4Compressor`
- **Constraints**: Operates on the `broker.MessageBroker` interface. Never references Kafka or RabbitMQ directly. Must remain broker-agnostic.
- **Dependencies**: `pkg/broker` only.

## Coordination Rules

### Dependency Direction

All dependencies flow inward toward `pkg/broker`. No lateral dependencies exist between sibling packages.

```
producer --> broker <-- consumer
              ^  ^
              |  |
          kafka  rabbitmq
```

### Interface Contracts

1. **MessageBroker** is the single integration point. Any new broker adapter must implement it fully: `Connect`, `Close`, `HealthCheck`, `IsConnected`, `Publish`, `Subscribe`, `Unsubscribe`, `Type`.
2. **Subscription** returned by `Subscribe` must support `Unsubscribe`, `IsActive`, `Topic`, `ID`.
3. **Handler** is `func(ctx context.Context, msg *broker.Message) error`. All consumer utilities wrap this type.
4. **Serializer** and **Compressor** are open extension points in the producer package. New formats plug in without modifying existing code.

### Adding a New Broker Adapter

1. Create `pkg/<brokername>/` with `Config`, `Producer`, `Consumer` types.
2. Add a new `BrokerType` constant in `pkg/broker/broker.go`.
3. Both `Producer` and `Consumer` must expose `Connect`, `Close`, `IsConnected`, `Publish` or `Subscribe`, `Unsubscribe`, and `Config()` methods.
4. Use injectable function pattern (`SetPublishFunc`) to avoid importing client libraries.
5. Write table-driven tests using `InMemoryBroker` or injectable functions.

### Error Handling Protocol

- All agents must use `broker.BrokerError` for structured errors and `broker.NewBrokerError` to construct them.
- Sentinel errors (`broker.ErrNotConnected`, `broker.ErrMessageInvalid`, etc.) are for quick comparison.
- `broker.IsRetryableError` determines retry eligibility. The consumer agent's `RetryPolicy` and `WithRetry` depend on this.
- `broker.MultiError` aggregates errors during batch teardown (e.g., `ConsumerGroup.Stop`).

### Testing Protocol

- All tests run with `go test ./... -count=1 -race`.
- No external infrastructure is required. `InMemoryBroker` and injectable functions cover all paths.
- Use `github.com/stretchr/testify` for assertions.
- Table-driven test style: `Test<Type>_<Method>_<Scenario>`.

### Concurrency Model

- `InMemoryBroker` uses `sync.RWMutex` for thread-safe queue and subscriber access. Message delivery to subscribers runs in separate goroutines.
- Kafka and RabbitMQ adapters use `sync/atomic` for connection state (`atomic.Bool`) and counters (`atomic.Int64`). Subscription maps are guarded by `sync.RWMutex`.
- `AsyncProducer` uses a buffered channel as a send queue with a dedicated goroutine for the send loop. Drains on stop.
- `BatchConsumer` uses a mutex-guarded buffer with a ticker-based flush loop. Context cancellation triggers a final flush.
- `ConsumerGroup` tracks running state with `atomic.Bool` and guards subscription maps with `sync.RWMutex`.

## Communication Between Agents

Agents do not communicate directly. All coordination happens through the `broker.MessageBroker` interface. A consumer agent and a producer agent can operate on the same broker instance concurrently. The broker implementation is responsible for thread safety.

## Release Checklist

1. All tests pass: `go test ./... -count=1 -race`
2. No vet warnings: `go vet ./...`
3. Formatting applied: `gofmt -w .`
4. Dependencies tidy: `go mod tidy`
5. CHANGELOG.md updated
6. Tag follows semver: `v0.1.0`, `v0.2.0`, etc.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->


<!-- CONST-035 anti-bluff addendum (cascaded) -->

## CONST-035 — Anti-Bluff Tests & Challenges (mandatory; inherits from root)

Tests and Challenges in this submodule MUST verify the product, not
the LLM's mental model of the product. A test that passes when the
feature is broken is worse than a missing test — it gives false
confidence and lets defects ship to users. Functional probes at the
protocol layer are mandatory:

- TCP-open is the FLOOR, not the ceiling. Postgres → execute
  `SELECT 1`. Redis → `PING` returns `PONG`. ChromaDB → `GET
  /api/v1/heartbeat` returns 200. MCP server → TCP connect + valid
  JSON-RPC handshake. HTTP gateway → real request, real response,
  non-empty body.
- Container `Up` is NOT application healthy. A `docker/podman ps`
  `Up` status only means PID 1 is running; the application may be
  crash-looping internally.
- No mocks/fakes outside unit tests (already CONST-030; CONST-035
  raises the cost of a mock-driven false pass to the same severity
  as a regression).
- Re-verify after every change. Don't assume a previously-passing
  test still verifies the same scope after a refactor.
- Verification of CONST-035 itself: deliberately break the feature
  (e.g. `kill <service>`, swap a password). The test MUST fail. If
  it still passes, the test is non-conformant and MUST be tightened.

## CONST-033 clarification — distinguishing host events from sluggishness

Heavy container builds (BuildKit pulling many GB of layers, parallel
podman/docker compose-up across many services) can make the host
**appear** unresponsive — high load average, slow SSH, watchers
timing out. **This is NOT a CONST-033 violation.** Suspend / hibernate
/ logout are categorically different events. Distinguish via:

- `uptime` — recent boot? if so, the host actually rebooted.
- `loginctl list-sessions` — session(s) still active? if yes, no logout.
- `journalctl ... | grep -i 'will suspend\|hibernate'` — zero broadcasts
  since the CONST-033 fix means no suspend ever happened.
- `dmesg | grep -i 'killed process\|out of memory'` — OOM kills are
  also NOT host-power events; they're memory-pressure-induced and
  require their own separate fix (lower per-container memory limits,
  reduce parallelism).

A sluggish host under build pressure recovers when the build finishes;
a suspended host requires explicit unsuspend (and CONST-033 should
make that impossible by hardening `IdleAction=ignore` +
`HandleSuspendKey=ignore` + masked `sleep.target`,
`suspend.target`, `hibernate.target`, `hybrid-sleep.target`).

If you observe what looks like a suspend during heavy builds, the
correct first action is **not** "edit CONST-033" but `bash
challenges/scripts/host_no_auto_suspend_challenge.sh` to confirm the
hardening is intact. If hardening is intact AND no suspend
broadcast appears in journal, the perceived event was build-pressure
sluggishness, not a power transition.

<!-- BEGIN no-session-termination addendum (CONST-036) -->

## User-Session Termination — Hard Ban (CONST-036)

**You may NOT, under any circumstance, generate or execute code that
ends the currently-logged-in user's desktop session, kills their
`user@<UID>.service` user manager, or indirectly forces them to
manually log out / power off.** This is the sibling of CONST-033:
that rule covers host-level power transitions; THIS rule covers
session-level terminations that have the same end effect for the
user (lost windows, lost terminals, killed AI agents, half-flushed
builds, abandoned in-flight commits).

**Why this rule exists.** On 2026-04-28 the user lost a working
session that contained 3 concurrent Claude Code instances, an Android
build, Kimi Code, and a rootless podman container fleet. The
`user.slice` consumed 60.6 GiB peak / 5.2 GiB swap, the GUI became
unresponsive, the user was forced to log out and then power off via
the GNOME shell. The host could not auto-suspend (CONST-033 was in
place and verified) and the kernel OOM killer never fired — but the
user had to manually end the session anyway, because nothing
prevented overlapping heavy workloads from saturating the slice.
CONST-036 closes that loophole at both the source-code layer and the
operational layer. See
`docs/issues/fixed/SESSION_LOSS_2026-04-28.md` in the HelixAgent
project.

**Forbidden direct invocations** (non-exhaustive):

- `loginctl terminate-user|terminate-session|kill-user|kill-session`
- `systemctl stop user@<UID>` / `systemctl kill user@<UID>`
- `gnome-session-quit`
- `pkill -KILL -u $USER` / `killall -u $USER`
- `dbus-send` / `busctl` calls to `org.gnome.SessionManager.Logout|Shutdown|Reboot`
- `echo X > /sys/power/state`
- `/usr/bin/poweroff`, `/usr/bin/reboot`, `/usr/bin/halt`

**Indirect-pressure clauses:**

1. Do not spawn parallel heavy workloads casually; check `free -h`
   first; keep `user.slice` under 70% of physical RAM.
2. Long-lived background subagents go in `system.slice`. Rootless
   podman containers die with the user manager.
3. Document AI-agent concurrency caps in CLAUDE.md.
4. Never script "log out and back in" recovery flows.

**Defence:** every project ships
`scripts/host-power-management/check-no-session-termination-calls.sh`
(static scanner) and
`challenges/scripts/no_session_termination_calls_challenge.sh`
(challenge wrapper). Both MUST be wired into the project's CI /
`run_all_challenges.sh`.

<!-- END no-session-termination addendum (CONST-036) -->

<!-- BEGIN const035-strengthening-2026-04-29 -->

## CONST-035 — End-User Usability Mandate (2026-04-29 strengthening)

A test or Challenge that PASSES is a CLAIM that the tested behavior
**works for the end user of the product**. The HelixAgent project
has repeatedly hit the failure mode where every test ran green AND
every Challenge reported PASS, yet most product features did not
actually work — buggy challenge wrappers masked failed assertions,
scripts checked file existence without executing the file,
"reachability" tests tolerated timeouts, contracts were honest in
advertising but broken in dispatch. **This MUST NOT recur.**

Every PASS result MUST guarantee:

a. **Quality** — the feature behaves correctly under inputs an end
   user will send, including malformed input, edge cases, and
   concurrency that real workloads produce.
b. **Completion** — the feature is wired end-to-end from public
   API surface down to backing infrastructure, with no stub /
   placeholder / "wired lazily later" gaps that silently 503.
c. **Full usability** — a CLI agent / SDK consumer / direct curl
   client following the documented model IDs, request shapes, and
   endpoints SUCCEEDS without having to know which of N internal
   aliases the dispatcher actually accepts.

A passing test that doesn't certify all three is a **bluff** and
MUST be tightened, or marked `t.Skip("...SKIP-OK: #<ticket>")`
so absence of coverage is loud rather than silent.

### Bluff taxonomy (each pattern observed in HelixAgent and now forbidden)

- **Wrapper bluff** — assertions PASS but the wrapper's exit-code
  logic is buggy, marking the run FAILED (or the inverse: assertions
  FAIL but the wrapper swallows them). Every aggregating wrapper MUST
  use a robust counter (`! grep -qs "|FAILED|" "$LOG"` style) —
  never inline arithmetic on a command that prints AND exits
  non-zero.
- **Contract bluff** — the system advertises a capability but
  rejects it in dispatch. Every advertised capability MUST be
  exercised by a test or Challenge that actually invokes it.
- **Structural bluff** — `check_file_exists "foo_test.go"` passes
  if the file is present but doesn't run the test or assert anything
  about its content. File-existence checks MUST be paired with at
  least one functional assertion.
- **Comment bluff** — a code comment promises a behavior the code
  doesn't actually have. Documentation written before / about code
  MUST be re-verified against the code on every change touching the
  documented function.
- **Skip bluff** — `t.Skip("not running yet")` without a
  `SKIP-OK: #<ticket>` marker silently passes. Every skip needs the
  marker; CI fails on bare skips.

The taxonomy is illustrative, not exhaustive. Every Challenge or
test added going forward MUST pass an honest self-review against
this taxonomy before being committed.

<!-- END const035-strengthening-2026-04-29 -->
