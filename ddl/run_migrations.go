package ddl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"api.audius.co/config"
)

func RunMigrations() error {
	if !config.Cfg.RunMigrations {
		fmt.Println("Skipping migrations. Set env runMigrations=true to run.")
		return nil
	}

	_, file, _, ok := runtime.Caller(0)
	migrationsDir := "ddl"
	if ok {
		migrationsDir = filepath.Dir(file)
	}

	cmd := exec.Command("bash", "pg_migrate.sh")
	cmd.Dir = migrationsDir

	// Ensure psql is discoverable: if not on PATH, add common Homebrew locations
	pathEnv := os.Getenv("PATH")
	if _, err := exec.LookPath("psql"); err != nil {
		candidates := []string{"/opt/homebrew/opt/libpq/bin", "/usr/local/opt/libpq/bin"}
		for _, c := range candidates {
			if _, statErr := os.Stat(filepath.Join(c, "psql")); statErr == nil {
				if !strings.Contains(pathEnv, c) {
					pathEnv = c + string(os.PathListSeparator) + pathEnv
				}
				break
			}
		}
	}

	cmd.Env = append(os.Environ(),
		"DB_URL="+config.Cfg.WriteDbUrl,
		"PATH="+pathEnv,
	)

	out, err := cmd.CombinedOutput()
	fmt.Println("pg_migrate.sh: ", string(out))
	return err
}
