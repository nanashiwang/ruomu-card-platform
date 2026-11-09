// Package invoicemigrate provides the explicit, additive upgrade for existing installations.
package invoicemigrate

import (
	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/models"
	"gorm.io/gorm"
)

// Apply creates only the invoice table and the three finance policies added in this release.
// Run against a backup first, with application writers stopped. It is safe to repeat.
func Apply(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&models.OrderInvoice{}); err != nil {
			return err
		}
		svc, err := authz.NewService(tx)
		if err != nil {
			return err
		}
		for _, action := range []string{"review", "file", "send"} {
			if err = svc.GrantRolePolicy("finance", "/admin/invoices/:id/"+action, "POST"); err != nil {
				return err
			}
		}
		return nil
	})
}
