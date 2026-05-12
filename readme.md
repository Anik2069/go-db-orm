# GoDB ORM 🚀

**A schema-first, type-safe, and migration-aware ORM for Go.**

GoDB ORM is inspired by the developer experience of Prisma and Laravel, bringing a powerful yet simple workflow to Go developers. It automates schema migrations, handles relationships, and generates type-safe Go models from simple `.schema` files.

---

## ✨ Features

- **Schema-First Workflow**: Define your models in simple `.schema` files.
- **Auto-Migrations**: Detects changes and generates versioned SQL migration files automatically.
- **Migration History**: Tracks applied migrations in a `migrations` table (multi-developer friendly).
- **Type-Safe Models**: Generates Go structs with appropriate types (including pointers for nullable fields).
- **Smart Naming**: Automatic snake_case and **Pluralization** (e.g., `User` -> `users`).
- **Foreign Keys**: Simple `@foreign(Table.column)` syntax for database relationships.
- **Reserved Word Safety**: Automatically quotes identifiers for PostgreSQL and MySQL.
- **Zero-Flag CLI**: Uses `godborm.json` for effortless local development.

---

## 🚀 Quick Start

### 1. Install the CLI
```bash
go install github.com/Anik2069/go-db-orm/cmd/godborm@latest
```

### 2. Initialize your project
```bash
godborm init
```
This creates a `godborm.json` config and a `/schema` folder.

### 3. Define your Schema
Create a file at `schema/user.schema`:
```prisma
model User {
    id         int      @id
    name       string
    email      string
    city       string?  // Nullable field
    
    created_at datetime
}

model Post {
    id      int      @id
    title   string
    user_id int      @foreign(User.id)
}
```

### 4. Run Migrations
```bash
godborm migrate
```
This generates SQL files in `/migrations` and syncs your database.

### 5. Generate Go Models
```bash
godborm generate
```
This creates a `models_gen.go` file with your Go structs.

---

## 🛠 Configuration (`godborm.json`)

```json
{
    "schema": "./schema",
    "migrations": "./migrations",
    "driver": "postgres",
    "dsn": "postgresql://user:pass@localhost:5432/dbname?sslmode=disable"
}
```

---

## 📖 Schema Syntax

| Attribute | Description |
| :--- | :--- |
| `int` | Maps to `INT` or `SERIAL` |
| `string` | Maps to `VARCHAR(255)` |
| `datetime` | Maps to `TIMESTAMP` or `DATETIME` |
| `type?` | Makes the field **nullable** (Go pointer) |
| `@id` | Sets the Primary Key |
| `@foreign(T.c)` | Sets a Foreign Key relationship |

---

## 🤝 Contributing
Contributions are welcome! Feel free to open issues or submit PRs.

## 📄 License
MIT
