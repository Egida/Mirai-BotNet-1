package mysql

import (
	"testing"
	"time"
)

func TestStaleConnectionChecks(t *testing.T) {
	runTests(t, dsn, func(dbt *DBTest) {
		dbt.mustExec("SET @@SESSION.wait_timeout = 2")

		if err := dbt.db.Ping(); err != nil {
			dbt.Fatal(err)
		}

		// wait for MySQL to close our connection
		time.Sleep(3 * time.Second)

		tx, err := dbt.db.Begin()
		if err != nil {
			dbt.Fatal(err)
		}

		if err := tx.Rollback(); err != nil {
			dbt.Fatal(err)
		}
	})
}
