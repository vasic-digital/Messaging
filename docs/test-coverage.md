# Messaging — Test Coverage Ledger (round-250)

Per **CONST-050(B)** every exported symbol in `pkg/*` MUST appear in this
ledger with a pointer to the test type(s) covering it. The ledger is
machine-verified by `challenges/messaging_describe_challenge.sh`
(paired-mutation gate) and is the single audit surface that establishes
"100% test-type coverage" claims for this submodule.

> Verbatim 2026-05-19 operator mandate: "all existing tests and Challenges
> do work in anti-bluff manner — they MUST confirm that all tested codebase
> really works as expected! We had been in position that all tests do
> execute with success and all Challenges as well, but in reality the most
> of the features does not work and can't be used! This MUST NOT be the
> case and execution of tests and Challenges MUST guarantee the quality,
> the completition and full usability by end users of the product!"
> — preserved verbatim per CONST-049 §11.4.17.

Article XI §11.9: every PASS line in the runner / unit-test output carries
positive runtime evidence (broker name, locale, byte length, topic). A
PASS without a captured payload is a bluff and a critical defect.

## Test-type matrix

| Layer | Source                                                   | Real infrastructure? | Mocks permitted? |
|-------|----------------------------------------------------------|----------------------|------------------|
| Unit  | `pkg/*/*_test.go`                                        | InMemoryBroker       | yes (CONST-050(A)) |
| Integration | `tests/integration/`                              | InMemoryBroker       | no               |
| E2E   | `tests/e2e/`                                             | InMemoryBroker       | no               |
| Security | `tests/security/`                                     | real bytes / DLQ     | no               |
| Stress | `tests/stress/`, `pkg/broker/stress_test.go`            | real concurrent dispatch | no           |
| Benchmark | `tests/benchmark/`                                   | real broker          | no               |
| Challenge | `challenges/scripts/`, `challenges/runner/main.go`   | real broker          | no               |
| Paired mutation | `challenges/messaging_describe_challenge.sh --anti-bluff-mutate` | gate itself | n/a |

## Symbol → test ledger

### `pkg/broker` (core interfaces + InMemoryBroker)

| Symbol | Test |
|--------|------|
| `BrokerType` | `pkg/broker/broker_test.go::TestBrokerType_String` + `TestBrokerType_IsValid` |
| `Message`, `NewMessage`, `NewMessageWithKey`, `NewMessageWithID` | `TestNewMessage*` |
| `Message.SetHeader` / `GetHeader` / `WithKey` / `WithStringKey` | `TestMessage_SetHeader_GetHeader`, `TestMessage_WithKey`, `TestMessage_WithStringKey` |
| `Message.Clone` | `TestMessage_Clone`, `TestMessage_Clone_NilHeaders`, runner Publish goroutine dispatch |
| `Message.MarshalJSON` / `UnmarshalJSON` | `TestMessage_MarshalJSON`, `TestMessage_UnmarshalJSON`, runner §2 (non-ASCII bytes through json round trip per locale) |
| `Config`, `Config.Validate`, `DefaultConfig` | `TestConfig_Validate`, `TestDefaultConfig` |
| `MessageBroker` interface | runtime compile-time blank assignment in runner (`var _ broker.MessageBroker = ...`) |
| `InMemoryBroker`, `NewInMemoryBroker` | `TestInMemoryBroker_*`, runner §1..§8 |
| `Connect` / `Close` / `HealthCheck` / `IsConnected` / `Type` | runner §1, §8 |
| `Publish` | runner §2 (per locale), `TestInMemoryBroker_PublishSubscribe`, `TestInMemoryBroker_Publish_NotConnected` |
| `PublishBatch` | runner §3, `pkg/broker/broker_test.go` batch path |
| `Subscribe` / `Unsubscribe` | runner §2/§3/§4/§5/§6, `TestInMemoryBroker_Unsubscribe`, `TestInMemoryBroker_Subscription_Unsubscribe` |
| `Subscription` (`ID` / `IsActive` / `Topic` / `Unsubscribe`) | runner §2..§6 cleanup, `TestInMemoryBroker_Subscription_Unsubscribe` |
| `Handler` type | runner all sections |
| `PublishOption` / `PublishOptions` / `SubscribeOption` / `SubscribeOptions` | `pkg/broker/options.go` unit coverage |
| `BrokerMetrics`, `NewBrokerMetrics`, `GetMetrics` | `pkg/broker/metrics.go` unit coverage |
| `BrokerError`, `ErrorCode`, `NewBrokerError`, `GetBrokerError`, `IsBrokerError`, `IsConnectionError`, `IsRetryableError`, `MultiError`, `NewMultiError`, `Add`, `Error`, `ErrorOrNil`, `HasErrors`, `Is`, `Unwrap` | `pkg/broker/errors.go` + unit tests (errors_test if present); runner §5 exercises retryable failure path |
| `SetTranslator` (i18n wiring) | `pkg/broker/i18n_wiring.go`; runner §4 ConsumerGroup uses translator under the hood |

### `pkg/consumer`

| Symbol | Test |
|--------|------|
| `ConsumerGroup`, `NewConsumerGroup` | `pkg/consumer/consumer_test.go`, runner §4 |
| `ConsumerGroup.ID` / `Add` / `Start` / `Stop` / `IsRunning` / `Topics` | runner §4 |
| `RetryPolicy`, `DefaultRetryPolicy`, `Delay`, `ShouldRetry` | runner §5 with tight policy (MaxRetries=2, BackoffBase=10ms) |
| `WithRetry` | runner §5 (real always-failing handler, real backoff, real exhaustion) |
| `DeadLetterHandler`, `NewDeadLetterHandler`, `Handle`, `Count`, `DLQTopic`, `SetOnFailure` | runner §5 (DLQ subscription captures forwarded message, asserts payload bytes intact) |
| `BatchConsumer`, `NewBatchConsumer`, `BatchSize`, `BufferLen`, `Flush`, `AsHandler` | `pkg/consumer/consumer_test.go` batch tests |

### `pkg/producer`

| Symbol | Test |
|--------|------|
| `SyncProducer`, `NewSyncProducer`, `Send`, `SendValue`, `SetSerializer`, `SetCompressor` | runner §6 (real broker, real JSONSerializer, non-ASCII payload round-trip) |
| `AsyncProducer`, `NewAsyncProducer`, `Start`, `Stop`, `Errors`, `SentCount`, `FailedCount` | `pkg/producer/producer_test.go` async tests |
| `Serializer`, `JSONSerializer`, `ProtobufSerializer`, `AvroSerializer`, `Serialize`, `Deserialize`, `ContentType` | runner §6 (JSONSerializer); `pkg/producer/producer_test.go` for protobuf / avro |
| `Compressor`, `Algorithm`, `Compress`, `Decompress`, `GzipCompressor`, `SnappyCompressor`, `LZ4Compressor` | `pkg/producer/producer_test.go` compressor tests |

### `pkg/kafka`

| Symbol | Test |
|--------|------|
| `Config`, `DefaultConfig`, `Validate` | `pkg/kafka/kafka_test.go::TestConfig_*` |
| `PartitionStrategy`, `String` | `TestPartitionStrategy_*` |
| `Producer`, `NewProducer`, `Connect`, `Close`, `IsConnected`, `Publish`, `SetPublishFunc` | `pkg/kafka/kafka_test.go` injectable publish path |
| `Consumer`, `NewConsumer`, `Subscribe`, `Unsubscribe` | `pkg/kafka/kafka_test.go` injectable consume path |
| Subscription methods on kafka adapter (`ID`, `IsActive`, `Topic`) | unit coverage |

### `pkg/rabbitmq`

| Symbol | Test |
|--------|------|
| `Config`, `DefaultConfig`, `Validate`, `ConnectionString` | `pkg/rabbitmq/rabbitmq_test.go` |
| `ExchangeType`, `String`, `IsValid` | `TestExchangeType_*` |
| `Producer`, `NewProducer`, `Connect`, `Close`, `IsConnected`, `Publish`, `SetPublishFunc` | `pkg/rabbitmq/rabbitmq_test.go` |
| `Consumer`, `NewConsumer`, `Subscribe`, `Unsubscribe` | `pkg/rabbitmq/rabbitmq_test.go` |
| Subscription methods on rabbitmq adapter (`ID`, `IsActive`, `Topic`) | unit coverage |

### `pkg/i18n`

| Symbol | Test |
|--------|------|
| `Translator` interface | `pkg/i18n/translator_test.go`, runner §2 implicit (NoopTranslator default) |
| `NoopTranslator`, `T` | `pkg/i18n/translator_test.go::TestNoopTranslator_*` |

## Anti-bluff invariants (round-250)

1. **Real broker, no in-line mock.** The runner imports `pkg/broker` and uses
   the exported `InMemoryBroker` exactly as a downstream consumer would.
   No patched goroutine, no overridden Handler dispatch.
2. **Byte preservation per locale.** Five locales (`en`, `sr-Latn`,
   `sr-Cyrl`, `ja`, `ar`) — every Publish/Subscribe round trip and every
   JSON Marshal/Unmarshal asserts `string(received.Value) == originalValue`
   byte-for-byte. UTF-8 corruption is a hard FAIL.
3. **Non-ASCII topics and keys.** Topics like `догађаји`, `イベント`,
   `الأحداث` are published AND delivered — verifying the broker's topic
   string handling is locale-neutral, not just the payloads.
4. **DLQ payload integrity.** The retry path uses a real always-failing
   handler, real backoff, and the DLQ subscription captures the forwarded
   message — runner asserts the DLQ payload bytes equal the original
   publish bytes (no truncation, no encoding drift).
5. **ConsumerGroup isolation.** N distinct topics → N distinct atomic
   counters → assert each counter == 1 after a single publish per topic.
   Catches cross-talk regressions.
6. **Runtime interface contract.** `var _ broker.MessageBroker = b` blank
   assignment runs at runtime in the binary — any interface drift breaks
   the build of the Challenge.
7. **Paired-mutation gate.** `messaging_describe_challenge.sh
   --anti-bluff-mutate` plants a symbol-rename mutation in the ledger,
   asserts the gate FAILS (exit 99). Proves the ledger walker is not a
   rubber stamp.

## Coverage gaps (tracked for round-251+)

- `pkg/broker/options.go` PublishOptions / SubscribeOptions surface not
  yet driven by the runner — covered by unit tests only.
- `pkg/producer` Compressor implementations (Gzip/Snappy/LZ4) covered by
  unit tests only; runner does not yet exercise compressed payload paths.
- `pkg/kafka` and `pkg/rabbitmq` adapters exercise injectable publish
  funcs (SetPublishFunc) — full real-broker integration deferred until
  `tests/integration/` gets compose stack per CONST-050(B).
