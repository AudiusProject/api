package ddl

import (
	"fmt"
	"os"
	"os/exec"

	"bridgerton.audius.co/config"
)

func RunMigrations() error {
	if !config.Cfg.RunMigrations {
		fmt.Println("Skipping migrations")
		return nil
	}

	cmd := exec.Command("sh", "pg_migrate.sh")

	cmd.Env = append(os.Environ(),
		"DB_URL="+config.Cfg.WriteDbUrl,
	)

	out, err := cmd.CombinedOutput()
	fmt.Println("pg_migrate.sh: ", string(out))
	return err
}
