package invoicemigrate

import (
	"fmt"
	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestAdditiveInvoiceMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:migrate-invoice-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err = db.Exec("CREATE TABLE preserved_business (id INTEGER PRIMARY KEY, value TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("INSERT INTO preserved_business VALUES (1, 'unchanged')").Error; err != nil {
		t.Fatal(err)
	}
	auth, err := authz.NewService(db)
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.BootstrapBuiltinRoles(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"review", "file", "send"} {
		if err = auth.RevokeRolePolicy("finance", "/admin/invoices/:id/"+path, "POST"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err = Apply(db); err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasTable(&models.OrderInvoice{}) {
		t.Fatal("invoice table missing")
	}
	var value string
	db.Raw("SELECT value FROM preserved_business WHERE id=1").Scan(&value)
	if value != "unchanged" {
		t.Fatal(value)
	}
	var count int64
	db.Table("casbin_rule").Where("ptype = ? AND v0 = ? AND v1 LIKE ?", "p", "role:finance", "/admin/invoices/%").Count(&count)
	if count != 3 {
		t.Fatal("policies", count)
	}
}
