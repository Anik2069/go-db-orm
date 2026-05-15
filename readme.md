# GoDB ORM 🚀

**A schema-first, type-safe, and migration-aware ORM for Go.**

GoDB ORM is inspired by the developer experience of Prisma and Laravel, bringing a powerful yet simple workflow to Go developers. It automates schema migrations, handles relationships, and generates type-safe Go models from simple `.schema` files.

🌍 **Website**: [go-db-orm.vercel.app](Visit Official Website)


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
- **Generic Type-Safe API**: `client.Query[User]()` — compile-time checked queries, no `interface{}`, no casting.

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

### 6. Connect in your Code
You can manually connect or use `ConnectWithConfig()` to automatically load your `godborm.json` credentials.

**Option A: Using `godborm.json` (Recommended)**
```go
import "github.com/Anik2069/go-db-orm/godborm/client"

func main() {
    // Automatically reads driver and dsn from godborm.json
    err := client.ConnectWithConfig()
    if err != nil {
        panic(err)
    }
    defer client.Close()
}
```

**Option B: Manual Connection**
```go
// Useful for environment variables or dynamic configurations
err := client.Connect("postgres", "postgresql://user:pass@localhost:5432/dbname")
```



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

GoDB ORM provides a **generic type-safe API** (recommended) and a classic reflection-based API for backward compatibility.

---

### ✅ Ideal Type-Safe API (Recommended)

Using `client.Model(&User{})` provides the best developer experience with **automatic type inference** and **compile-time safety**.

```go
// Fetch all matching rows — returns []User
users, err := client.Model(&User{}).
    Where("city = ?", "Dhaka").
    Where("age > ?", 18).
    OrderBy("created_at DESC").
    Limit(10).
    Offset(20).
    Find()

// Fetch a single record by ID — returns *User
user, err := client.Model(&User{}).FindOne(1)

// Fetch first matching record — returns *User
user, err := client.Model(&User{}).Where("active = ?", true).First()

// Count matching records
count, err := client.Model(&User{}).Where("age > ?", 18).Count()
```

---

### ✅ Smart CRUD

```go
// Save handles both Create and Update automatically
user := &User{Name: "Alice", Email: "alice@example.com"}
err := client.Model(user).Save()   // INSERT (ID was 0)

user.Name = "Bob"
err = client.Model(user).Save()   // UPDATE (ID is now set)

// Delete
err = client.Model(user).Delete()
```


---

### ✅ Transactions

```go
// Option A: manual — full control
tx, err := client.Begin()
if err != nil { ... }
defer tx.Rollback() // no-op after Commit

tx.Create(&order)
tx.Update(&inventory)

if err := tx.Commit(); err != nil { ... }

// Option B: WithTx — auto commit/rollback + panic recovery
err := client.WithTx(func(tx *client.Tx) error {
    if err := tx.Create(&order); err != nil {
        return err   // triggers automatic rollback
    }
    if err := tx.Update(&inventory); err != nil {
        return err
    }
    return nil // triggers automatic commit
})
```

---

### ✅ Struct Mapping & NULL Safety

Fields are mapped by the `db:` tag (falling back to snake_case of the field name).
NULL columns are handled safely — no nil pointer panics.

```go
type User struct {
    ID        int     `db:"id"`
    Name      string  `db:"name"`
    Bio       *string `db:"bio"`       // nullable → nil when NULL in DB
    CreatedAt string  `db:"created_at"`
}
```

---

### Loading Relations

```go
// Eager-load related models
invoices, err := client.Query[Invoice]().
    Select("invoice_number").
    Include("Items", "User").
    All()

// Load only specific fields from a relation
invoices, err := client.Query[Invoice]().
    Include("Items:item_name,quantity", "User").
    All()
```

---

### Raw SQL Queries 🛠

```go
// Fetch into a typed slice
var users []User
client.Raw("SELECT * FROM users WHERE email LIKE $1", "%@gmail.com").Scan(&users)

// Fetch a scalar value
var count int
client.Raw("SELECT COUNT(*) FROM users").Scan(&count)

// Execute a statement
client.Raw("DELETE FROM users WHERE id = $1", 123).Exec()
```

---

## 🤝 Contributing
Contributions are welcome! Feel free to open issues or submit PRs.

## 📄 License
MIT


---

### ✅ Generic API (Recommended)

Using `client.Query[T]()` gives you **compile-time safety** — no `interface{}`, no type assertions, no runtime surprises.

**Fetch all rows:**
```go
users, err := client.Query[User]().All()
// users is []User — fully typed, no casting needed
```

**Fetch by primary key:**
```go
user, err := client.Query[User]().Find(1)
// user is *User
```

**Fetch the first match:**
```go
user, err := client.Query[User]().First()
```

**Chaining Where / OrderBy / Limit / Offset:**
```go
users, err := client.Query[User]().
    Where("city = ?", "Dhaka").
    Where("age > ?", 18).
    OrderBy("created_at DESC").
    OrderBy("name ASC").   // multi-column: ORDER BY created_at DESC, name ASC
    Limit(10).
    Offset(20).
    All()
```

**Select specific columns:**
```go
users, err := client.Query[User]().
    Select("name", "email").
    Where("active = ?", true).
    All()
```

**Type-safe CRUD:**
```go
// Create
user := &User{Name: "Alice", Email: "alice@example.com"}
client.CreateOne(user)

// Update
user.Name = "Bob"
client.UpdateOne(user)

// Delete by ID
client.DeleteOne[User](1)
```

---

### Classic API (Reflection-based)

Still supported for backward compatibility.

```go
var users []User
client.Where("city = ?", "Dhaka").OrderBy("created_at DESC").Limit(10).FindAll(&users)
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

### Raw SQL Queries 🛠
Sometimes you need full control. Use `.Raw()` to execute arbitrary SQL:

```go
// Fetch into a slice of structs
var users []User
client.Raw("SELECT * FROM users WHERE email LIKE $1", "%@gmail.com").Scan(&users)

// Fetch a single value
var count int
client.Raw("SELECT COUNT(*) FROM users").Scan(&count)

// Execute a command (Update/Delete)
client.Raw("DELETE FROM users WHERE id = $1", 123).Exec()
```

---

---

## 🤝 Contributing
Contributions are welcome! Feel free to open issues or submit PRs.

## 📄 License
MIT
