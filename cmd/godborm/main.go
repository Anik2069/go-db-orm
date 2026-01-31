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

		_ = migrateCmd.Parse(args[1:])

		if *schemaPath == "" {
			log.Fatal("Please provide --schema path")
		}

		// Connect DB (from testapp)
		err := client.Connect("mysql", "root:@tcp(localhost:3306)/testdb")
		if err != nil {
			log.Fatal(err)
		}
		defer client.DB.Close()

		fmt.Println("Connected to MySQL successfully!")

		// Run migration
		err = godborm.Migrate(*schemaPath)
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
