package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"mime/multipart"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInvoiceAdminReviewUploadAndFailedMail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:invoice-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.AutoMigrate(&models.Order{}, &models.OrderInvoice{}); err != nil {
		t.Fatal(err)
	}
	svc := service.NewInvoiceService(db, service.NewEmailService(&config.EmailConfig{}))
	h := &Handler{Container: &provider.Container{InvoiceService: svc}}
	now := time.Now()
	o := models.Order{OrderNo: "HTTP-INVOICE", UserID: 12, Status: "paid", PaidAt: &now, Currency: "CNY", TotalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100))}
	if err = db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item, err := svc.Apply(o.ID, o.UserID, service.InvoiceInput{Kind: "personal", Title: "Test", Email: "invoice@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("admin_id", uint(9)) })
	router.GET("/", h.ListInvoices)
	router.POST("/:id/review", h.ReviewInvoice)
	router.POST("/:id/file", h.UploadInvoice)
	router.POST("/:id/send", h.SendInvoice)
	router.GET("/:id/download", h.DownloadInvoice)
	request := func(path, content string, body []byte, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", path, bytes.NewReader(body))
		r.Header.Set("Content-Type", content)
		router.ServeHTTP(w, r)
		var response struct {
			Code int `json:"status_code"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Code != want {
			t.Fatal(w.Body.String(), err)
		}
	}
	base := fmt.Sprintf("/%d", item.ID)
	request(base+"/review", "application/json", []byte(`{"decision":"unknown"}`), 400)
	request(base+"/review", "application/json", []byte(`{"decision":"approve"}`), 0)
	request(base+"/file", "text/plain", []byte("invalid"), 400)
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, _ := writer.CreateFormFile("file", "original.pdf")
	pdf := []byte("%PDF-1.4\ncontent\n%%EOF\n")
	part.Write(pdf)
	writer.Close()
	request(base+"/file", writer.FormDataContentType(), payload.Bytes(), 0)
	request(base+"/send", "application/json", nil, 400)
	stored, _ := svc.ForOrder(o.ID)
	if stored.MailStatus != "failed" || stored.Status != "issued" {
		t.Fatal(stored)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", base+"/download", nil))
	if !bytes.Equal(w.Body.Bytes(), pdf) || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/?status=issued&order_no=HTTP-INVOICE", nil))
	if bytes.Contains(w.Body.Bytes(), []byte("file_data")) || !bytes.Contains(w.Body.Bytes(), []byte(`"total":1`)) {
		t.Fatal(w.Body.String())
	}
}
