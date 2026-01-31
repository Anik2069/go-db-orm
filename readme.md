# go-db-orm

**A schema-first, code-generated, Go-idiomatic ORM for PostgreSQL.**  

`go-db-orm` provides a type-safe Go client for database operations, inspired by Prisma and Laravel Eloquent, but optimized for **large databases** and **developer productivity**.

---

## Features

- Schema-first, per-table design (Laravel-style)  
- Incremental code generation (generate only what you need)  
- Type-safe Go client (no string queries in app code)  
- Explicit, predictable, Go-idiomatic API  
- PostgreSQL-first, extensible to other databases  
- Ideal for large tables and legacy DBs

---

## Installation

```bash
go get github.com/Anik2069/go-db-orm/godborm


# schema/user.schema

model User {
    table users

    id    Int    @id @auto
    email String @unique
    name  String
}

