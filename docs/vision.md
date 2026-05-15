# go-db-orm Vision

## Problem Statement

Go developers face a common pain: working with ORMs in large-scale projects is costly and error-prone. Existing Go ORMs, like GORM or sqlx, have several limitations:

- Heavy reliance on reflection or struct tags leads to verbose and fragile code.
- String-based queries make code prone to runtime errors.
- Full-database introspection for code generation (like Prisma) becomes slow and memory-intensive for large databases.
- Handling large schemas with hundreds of tables is cumbersome and often slows down development.

These issues increase **developer time**, reduce productivity, and make large-scale Go projects harder to maintain.

---

## Our Solution

`go-db-orm` provides a **schema-first, code-generated, Go-idiomatic ORM** that addresses these problems:

1. **Per-table schema files**  
   Each table has its own schema file. No need to load the entire database. This keeps generation fast, even with very large databases.

2. **Incremental code generation**  
   Only changed or selected tables are generated. No unnecessary work, no global regeneration.

3. **Type-safe Go client**  
   Generated Go code is fully type-safe. No string queries in application code. This reduces runtime errors and improves DX (developer experience).

4. **Explicit, predictable, Go-idiomatic API**  
   All behavior is explicit, easy to read, and follows Go conventions. No hidden magic.

5. **PostgreSQL-first (extensible later)**  
   We start with PostgreSQL, but the architecture allows adding other database adapters later.

---

## Guiding Principles

- **Explicit over magic**: All database interactions are clear and predictable.  
- **Schema is the source of truth**: Schema files define tables and fields, not struct tags or runtime reflection.  
- **Modular generation**: Each table generates its own Go code.  
- **Incremental workflow**: Large databases are supported without performance bottlenecks.  
- **Open-source and community-driven**: Designed to be extensible, maintained, and improved by the Go community.

---

## v0.1 Scope

For the initial release (v0.1), we will focus on:

- **Schema**: Single-table schema files (`schema/*.schema`)  
- **Generation**: Generate Go structs and basic CRUD methods per table  
- **Database adapter**: PostgreSQL only  
- **Operations**: Insert, FindOne, FindMany, Update, Delete  
- **No relations, no migrations yet**  

This small but solid foundation ensures we deliver a usable package while keeping complexity under control.

---

## Future Vision

To close the gap with industry standards like GORM while maintaining our schema-first advantage, future versions will focus on:

- **Advanced Associations**: Full support for `many-to-many` relationships and polymorphic associations.
- **Query Power**: Implementing complex `JOIN` logic, subqueries, and `GROUP BY / HAVING` clauses within the type-safe API.
- **Lifecycle Hooks & Middleware**: A robust plugin system for `Before/After` hooks (Create, Update, Delete, Find).
- **Soft Deletes**: Native support for soft deletion patterns.
- **Expanded Database Support**: Adding SQLite, SQL Server, and other major SQL dialects.
- **Observability**: Built-in plugins for OpenTelemetry tracing, Prometheus metrics, and advanced logging.
- **Advanced Transactions**: Support for nested transactions and savepoints.


By following this roadmap, `go-db-orm` aims to bring **developer productivity and type safety** to Go ORMs at scale.

---

**Goal:** Make Go database development **faster, safer, and more predictable**, without compromising on the clarity and simplicity that Go developers expect.
