package ddl

import (
	"fmt"
	"os"
	"os/exec"

	"api.audius.co/config"
)

func RunMigrations(forceMigration bool) error {
	fmt.Println("Running migrations...")
	if !config.Cfg.RunMigrations && !forceMigration {
		fmt.Println("Skipping migrations. Set env runMigrations=true to run.")
		return nil
	}

	cmd := exec.Command("bash", "pg_migrate.sh")
	cmd.Dir = "ddl"

	cmd.Env = append(os.Environ(),
		"DB_URL="+config.Cfg.WriteDbUrl,
	)

	out, err := cmd.CombinedOutput()
	fmt.Println("pg_migrate.sh: ", string(out))
	if err != nil {
		fmt.Println("Error running pg_migrate.sh:", err)
	}
	return err
}
