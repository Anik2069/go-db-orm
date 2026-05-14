package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Anik2069/go-db-orm/godborm"
	"github.com/Anik2069/go-db-orm/godborm/client"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	SchemaPath     string `json:"schema"`
	MigrationsPath string `json:"migrations"`
	Driver         string `json:"driver"`
	DSN            string `json:"dsn"`
}

func loadConfig() *Config {
	data, err := os.ReadFile("godborm.json")
	if err != nil {
		return &Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{}
	}
	return &cfg
}

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

	cfg := loadConfig()

	switch args[0] {
	case "init":
		// Create default config
		defaultCfg := Config{
			SchemaPath:     "./schema",
			MigrationsPath: "./migrations",
			Driver:         "mysql",
			DSN:            "root:@tcp(localhost:3306)/dbname?parseTime=true",
		}
		data, _ := json.MarshalIndent(defaultCfg, "", "    ")
		if err := os.WriteFile("godborm.json", data, 0o644); err != nil {
			log.Fatalf("failed to create godborm.json: %v", err)
		}

		// Create schema directory
		if err := os.MkdirAll("./schema", 0o755); err != nil {
			log.Fatalf("failed to create schema directory: %v", err)
		}

		fmt.Println("Project initialized successfully!")
		fmt.Println("1. Edit godborm.json with your DB credentials.")
		fmt.Println("2. Add your models to the /schema directory.")
		fmt.Println("3. Run 'godborm migrate' to start.")

	case "migrate":
		migrateCmd := flag.NewFlagSet("migrate", flag.ExitOnError)

		// Set defaults from config if available
		defSchema := cfg.SchemaPath
		defMigrations := cfg.MigrationsPath
		if defMigrations == "" {
			defMigrations = "./migrations"
		}
		defDriver := os.Getenv("DB_DRIVER")
		if defDriver == "" {
			defDriver = cfg.Driver
		}
		defDSN := os.Getenv("DB_DSN")
		if defDSN == "" {
			defDSN = cfg.DSN
		}

		schemaPath := migrateCmd.String("schema", defSchema, "Path to schema folder")
		migrationsPath := migrateCmd.String("migrations", defMigrations, "Path to save generated migration files")
		driver := migrateCmd.String("driver", defDriver, "Database driver (mysql or postgres)")
		dsn := migrateCmd.String("dsn", defDSN, "Database connection string (DSN)")

		_ = migrateCmd.Parse(args[1:])

		if *schemaPath == "" {
			log.Fatal("Please provide --schema path or set in godborm.json")
		}
		if *driver == "" || *dsn == "" {
			log.Fatal("Please provide --driver and --dsn (or set in godborm.json / environment variables)")
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

		defSchema := cfg.SchemaPath
		schemaPath := genCmd.String("schema", defSchema, "Path to schema folder")
		packageName := genCmd.String("package", "models", "Package name for generated structs")
		outPath := genCmd.String("out", "./models_gen.go", "Output .go file path")

		_ = genCmd.Parse(args[1:])

		if *schemaPath == "" {
			log.Fatal("Please provide --schema path or set in godborm.json")
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
