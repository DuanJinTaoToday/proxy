// Command dao generates models from the migrated development database.
package main

import (
	"database/sql"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("PROXY_CONTROL_DAO_DSN")
	if dsn == "" {
		log.Fatal("set PROXY_CONTROL_DAO_DSN to the read-only development database connection")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("could not connect to the development database")
	}
	if err := db.Transaction(generateModels, &sql.TxOptions{ReadOnly: true}); err != nil {
		log.Fatal(err)
	}
}

func generateModels(db *gorm.DB) error {
	g := gen.NewGenerator(gen.Config{
		OutPath:          "internal/data/query",
		ModelPkgPath:     "model",
		FieldNullable:    true,
		FieldCoverable:   true,
		FieldWithTypeTag: true,
	})
	g.UseDB(db)
	for _, name := range []string{
		"gateways", "gateway_tokens", "proxy_credentials", "gateway_auth_references",
		"tenant_groups", "tenant_group_members", "billing_settings",
		"traffic_grants", "tenant_balances",
	} {
		g.GenerateModel(name)
	}
	g.Execute()
	return nil
}
