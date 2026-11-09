package public

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInvoiceUserAndTenantIsolation(t *testing.T) {
	db := openPublicResellerHandlerTestDB(t)
	if err := db.AutoMigrate(&models.OrderInvoice{}, &models.Fulfillment{}); err != nil {
		t.Fatal(err)
	}
	s := service.NewInvoiceService(db, nil)
	h := &Handler{Container: &provider.Container{InvoiceService: s, OrderService: service.NewOrderService(service.OrderServiceOptions{OrderRepo: repository.NewOrderRepository(db)})}}
	now := time.Now()
	reseller := uint(34)
	o := models.Order{OrderNo: "invoice-private", UserID: 123, ResellerID: &reseller, Status: "paid", PaidAt: &now, Currency: "CNY", TotalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100))}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item, err := s.Apply(o.ID, o.UserID, service.InvoiceInput{Kind: "personal", Title: "购买人", Email: "invoice@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Review(item.ID, 1, true, ""); err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nprivate document\n%%EOF\n")
	if err = s.Upload(item.ID, 1, pdf); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		user    uint
		tenant  service.TenantContext
		allowed bool
	}{
		{"owner", 123, service.ResellerTenantContext("example.test", 34, 9, "example.test"), true},
		{"other_user", 999, service.ResellerTenantContext("example.test", 34, 9, "example.test"), false},
		{"main_site", 123, service.MainTenantContext(""), false},
		{"other_reseller", 123, service.ResellerTenantContext("other.test", 35, 9, "other.test"), false},
	} {
		for _, operation := range []string{"get", "download", "apply"} {
			t.Run(tc.name+"/"+operation, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Set("user_id", tc.user)
				c.Params = gin.Params{{Key: "order_no", Value: o.OrderNo}}
				c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"kind":"personal","title":"test","email":"other@example.test"}`))
				c.Request.Header.Set("Content-Type", "application/json")
				c.Request = c.Request.WithContext(service.WithTenantContext(c.Request.Context(), tc.tenant))
				switch operation {
				case "get":
					h.GetInvoice(c)
				case "download":
					h.DownloadInvoice(c)
				case "apply":
					h.ApplyInvoice(c)
				}
				if operation == "download" && tc.allowed {
					if !bytes.Equal(w.Body.Bytes(), pdf) || w.Header().Get("Cache-Control") != "private, no-store" {
						t.Fatal(w.Body.String())
					}
					return
				}
				var response struct {
					Status int             `json:"status_code"`
					Data   json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err, w.Body.String())
				}
				if !tc.allowed && response.Status != 404 {
					t.Fatal("unauthorized access", response.Status, w.Body.String())
				}
				if tc.allowed && operation == "get" && response.Status != 0 {
					t.Fatal(response)
				}
				if tc.allowed && operation == "apply" && response.Status != 400 {
					t.Fatal("duplicate accepted", response)
				}
				if !tc.allowed && bytes.Contains(w.Body.Bytes(), []byte("invoice@example.test")) {
					t.Fatal("PII leaked")
				}
			})
		}
	}
	// A missing login cannot read invoice metadata.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Params = gin.Params{{Key: "order_no", Value: o.OrderNo}}
	h.GetInvoice(c)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"status_code":401`)) {
		t.Fatal(fmt.Sprint(w.Body))
	}
}
