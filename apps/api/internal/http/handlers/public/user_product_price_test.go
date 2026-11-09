package public

import (
	"fmt"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUserProductPriceDisplayIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:userprices_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.UserProductPrice{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewUserProductPriceRepository(db)
	price := models.NewMoneyFromDecimal(decimal.NewFromInt(120))
	if err = repo.Upsert(&models.UserProductPrice{UserID: 7, ProductID: 1, SKUID: 11, PriceAmount: price}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Container: &provider.Container{UserProductPriceService: service.NewUserProductPriceService(repo, nil, nil, nil)}}
	for _, tc := range []struct {
		name     string
		user     uint
		reseller bool
		want     string
	}{
		{"own", 7, false, "120.00"}, {"other", 8, false, "100.00"}, {"anonymous-query-cannot-select-user", 0, false, "100.00"}, {"reseller", 7, true, "100.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			product := &models.Product{ID: 1, PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)), SKUs: []models.ProductSKU{{ID: 11, IsActive: true, PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100))}, {ID: 12, IsActive: true, PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(200))}}}
			view, err := h.decoratePublicProduct(product, nil)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/api/v1/public/products?user_id=7", nil)
			if tc.user > 0 {
				c.Set("user_id", tc.user)
			}
			if tc.reseller {
				c.Request = c.Request.WithContext(service.WithTenantContext(c.Request.Context(), service.ResellerTenantContext("shop.example", 1, 9, "shop.example")))
			}
			if err = h.applyUserProductPrices(c, &view, 11); err != nil {
				t.Fatal(err)
			}
			if view.PriceAmount.String() != tc.want || view.SKUs[0].PriceAmount.String() != tc.want || view.SKUs[1].PriceAmount.String() != "200.00" {
				t.Fatalf("incorrect display %+v", view)
			}
			if view.UserPrice != (tc.name == "own") {
				t.Fatal("incorrect user-price marker")
			}
			if tc.name == "own" && w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("missing private cache protection")
			}
			if product.SKUs[0].PriceAmount.String() != "100.00" {
				t.Fatal("mutated shared product")
			}
		})
	}
	if err = db.Migrator().DropTable(&models.UserProductPrice{}); err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_id", uint(7))
	c.Request = httptest.NewRequest("GET", "/products", nil)
	product := &models.Product{ID: 1, SKUs: []models.ProductSKU{{ID: 11, IsActive: true}}}
	view, _ := h.decoratePublicProduct(product, nil)
	if err = h.applyUserProductPrices(c, &view, 11); err == nil {
		t.Fatal("storage failure silently ignored")
	}
}
