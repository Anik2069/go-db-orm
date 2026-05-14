# GoDB ORM 🚀

**A schema-first, type-safe, and migration-aware ORM for Go.**

GoDB ORM is inspired by the developer experience of Prisma and Laravel, bringing a powerful yet simple workflow to Go developers. It automates schema migrations, handles relationships, and generates type-safe Go models from simple `.schema` files.

---

## ✨ Features

- **Schema-First Workflow**: Define your models in simple `.schema` files.
- **Auto-Migrations**: Detects changes and generates versioned SQL migration files automatically.
- **Migration History**: Tracks applied migrations in a `migrations` table (multi-developer friendly).
- **Type-Safe Models**: Generates Go structs with appropriate types (including pointers for nullable fields).
- **Smart Naming**: 
    - **DB**: Automatic snake_case and **Pluralization** (e.g., `User` -> `users`).
    - **Go**: Automatic **PascalCase** conversion (e.g., `created_at` -> `CreatedAt`).
- **Foreign Keys**: Simple `@foreign(Table.column)` syntax for database relationships.
- **Reserved Word Safety**: Automatically quotes identifiers for PostgreSQL and MySQL.
- **Smart Relation Loading**: 
    - **Filtered Includes**: Select specific fields for relations using `Relation:col1,col2` syntax.
    - **Auto-Linking**: Automatically fetches required join keys for relations if missing from `Select()`.
    - **Clean JSON**: Respects `Select()` by hiding internal join keys from final JSON output.
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
    city       string?  // Nullable field in DB, *string in Go
    
    created_at datetime // CreatedAt in Go
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
# Default package is "models", use --package main if running in root
godborm generate --package main
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

## 🔍 Querying & Relations

GoDB ORM provides a powerful and intuitive API for fetching data and its relationships.

### Basic Query
```go
var users []User
// Only fetches name and email
client.Select("name", "email").FindAll(&users)
```

### Loading Relations
You can load related models using `.Include()`. The ORM is smart enough to fetch the necessary join keys automatically, even if you don't select them.

```go
var invoices []Invoices
// Automatically fetches 'id' and 'user_id' internally to link Items and User
client.Select("invoice_number").Include("Items", "User").FindAll(&invoices)
```

### Filtered Relations ⚡
Control exactly which fields are fetched for related models using the `:` syntax:

```go
// Only fetch item_name and quantity for the Items relation
client.Select("invoice_number").Include("Items:item_name,quantity", "User").FindAll(&invoices)
```

### Clean JSON Output
Internal fields (like join keys added for relations) are automatically zeroed out after relations are loaded. This allows Go's `json:"...,omitempty"` to hide them, keeping your API responses clean and respecting your `Select()` intent.

---

---

## 🤝 Contributing
Contributions are welcome! Feel free to open issues or submit PRs.

## 📄 License
MIT
