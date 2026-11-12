// Command gen generates models from the migrated development database.
package main

import (
	"database/sql"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// databaseDSN points to the migrated development database. Use a read-only account.
const databaseDSN = ""

func main() {
	if databaseDSN == "" {
		log.Fatal("set databaseDSN in internal/data/gen/gen.go to the read-only development database connection")
	}
	db, err := gorm.Open(postgres.Open(databaseDSN), &gorm.Config{})
	if err != nil {
		log.Fatal("could not connect to the development database")
	}
	if err := db.Transaction(generateModels, &sql.TxOptions{ReadOnly: true}); err != nil {
		log.Fatal(err)
	}
}

func generateModels(db *gorm.DB) error {
	g := gen.NewGenerator(gen.Config{
		OutPath:          "internal/data/repo/query",
		ModelPkgPath:     "model",
		FieldNullable:    true,
		FieldCoverable:   true,
		FieldWithTypeTag: true,
	})
	g.UseDB(db)
	for _, name := range []string{
		"gateways", "gateway_tokens", "gateway_boots", "ingest_cursors",
		"proxy_credentials", "gateway_auth_references",
		"tenant_groups", "tenant_group_members", "billing_settings", "billing_rates",
		"traffic_grants", "tenant_balances", "balance_ledger",
		"usage_minutely", "usage_daily", "usage_archive_state", "record_anomalies",
	} {
		g.GenerateModel(name)
	}
	g.Execute()
	return nil
}
