# GoDB ORM for VS Code

![GoDB ORM Logo](icon.png)

This extension provides comprehensive support for **GoDB ORM** schema files (`.schema`, `.godb`).

## Features

- **Syntax Highlighting**: Full support for models, enums, types, and decorators.
- **Snippets**: Quick snippets for creating models and fields.
- **Language Configuration**: Intelligent bracket matching and comment support.

## Usage

Define your database models in `.schema` files:

```prisma
model User {
  id    int    @id
  name  string
  email string
}
```

## License

MIT
