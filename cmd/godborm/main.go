package main

import (
	"flag"
	"fmt"
	"go-db-orm/godborm"
	"go-db-orm/godborm/client"
	"log"
	"os"
	"path/filepath"
)

func main() {
	// Support: `godborm migrate --schema ./schema`
	// If no subcommand provided, default to migrate.
	args := os.Args[1:]

	// Allow an initial `--` (common when invoking via `go run ... -- ...`)
	for len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	if len(args) < 1 {
		log.Fatal("expected a subcommand, e.g. `godborm migrate --schema ./schema`")
	}

	switch args[0] {
	case "migrate":
		migrateCmd := flag.NewFlagSet("migrate", flag.ExitOnError)
		schemaPath := migrateCmd.String("schema", "", "Path to schema folder")
		migrationsPath := migrateCmd.String("migrations", "./migrations", "Path to save generated migration files")
		driver := migrateCmd.String("driver", os.Getenv("DB_DRIVER"), "Database driver (mysql or postgres)")
		dsn := migrateCmd.String("dsn", os.Getenv("DB_DSN"), "Database connection string (DSN)")

		_ = migrateCmd.Parse(args[1:])

		if *schemaPath == "" {
			log.Fatal("Please provide --schema path")
		}
		if *driver == "" || *dsn == "" {
			log.Fatal("Please provide --driver and --dsn (or set DB_DRIVER and DB_DSN environment variables)")
		}

		// Connect DB
		err := client.Connect(*driver, *dsn)
		if err != nil {
			log.Fatal(err)
		}
		defer client.DB.Close()

		// Run migration (generates file and pushes to DB)
		err = godborm.Migrate(*schemaPath, *migrationsPath)
		if err != nil {
			log.Fatal(err)
		}
	case "generate":
		genCmd := flag.NewFlagSet("generate", flag.ExitOnError)
		schemaPath := genCmd.String("schema", "", "Path to schema folder")
		packageName := genCmd.String("package", "models", "Package name for generated structs")
		outPath := genCmd.String("out", "./models_gen.go", "Output .go file path")

		_ = genCmd.Parse(args[1:])

		if *schemaPath == "" {
			log.Fatal("Please provide --schema path")
		}

		absOut, err := filepath.Abs(*outPath)
		if err != nil {
			log.Fatal(err)
		}

		if err := godborm.GenerateModels(*schemaPath, *packageName, absOut); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Models generated at %s\n", absOut)
	default:
		log.Fatalf("unknown subcommand: %s", args[0])
	}
}
