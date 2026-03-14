# SQL Definitions

## Module Overview

This module does not directly use SQL databases for its core functionality.

## Database Usage

### Primary Storage
- **Technology**: Kafka and RabbitMQ message brokers
- **Purpose**: Message queuing, publish/subscribe, dead letter queues, retry policies
- **Schema**: No SQL schema is required

### Optional SQL Integration
- Message persistence can be configured to use SQL databases as backing store
- Dead letter queue storage may use SQL tables for audit and recovery
- Consumer offset tracking can be stored in SQL databases (alternative to Kafka's __consumer_offsets)
- Message schema validation may reference SQL schemas

## Related Modules

For SQL database functionality, see the [Database module](../Database/README.md).

## Migration Notes

If SQL support is added in the future:
1. Create migration scripts in `migrations/` directory
2. Follow versioned migration pattern (`001_initial.sql`, `002_add_feature.sql`)
3. Use the `digital.vasic.database` module for database operations
4. Update this document with schema definitions