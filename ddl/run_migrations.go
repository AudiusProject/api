package ddl

import (
	"fmt"
	"os"
	"os/exec"

	"api.audius.co/config"
)

func RunMigrations() error {
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
