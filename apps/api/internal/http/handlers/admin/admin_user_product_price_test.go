package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdminUserProductPriceEndpoints(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:admin_userprices_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.Product{}, &models.ProductSKU{}, &models.UserProductPrice{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Email: "prices@example.test", PasswordHash: "hash", Status: "active"}
	if err = db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	product := models.Product{Slug: "user-price", TitleJSON: models.JSON{"zh-CN": "专属价商品"}}
	if err = db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	users := repository.NewUserRepository(db)
	h := &Handler{Container: &provider.Container{UserRepo: users, UserProductPriceService: service.NewUserProductPriceService(repository.NewUserProductPriceRepository(db), users, repository.NewProductRepository(db), repository.NewProductSKURepository(db))}}
	r := gin.New()
	r.GET("/users/:id/prices", h.GetUserProductPrices)
	r.PUT("/users/:id/prices", h.SetUserProductPrice)
	r.DELETE("/users/:id/prices/:price_id", h.DeleteUserProductPrice)
	call := func(method, path, body string) map[string]interface{} {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		var result map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	path := fmt.Sprintf("/users/%d/prices", user.ID)
	for _, amount := range []string{`"0"`, `"-1"`, `"1.001"`, `"1e1000000"`, `12.34`} {
		result := call("PUT", path, fmt.Sprintf(`{"product_id":%d,"price_amount":%s}`, product.ID, amount))
		if result["status_code"] == float64(0) {
			t.Fatal("accepted invalid money", amount)
		}
	}
	result := call("PUT", path, fmt.Sprintf(`{"product_id":%d,"price_amount":"12.34"}`, product.ID))
	if result["status_code"] != float64(0) {
		t.Fatal(result)
	}
	result = call("GET", path, "")
	rows := result["data"].([]interface{})
	if len(rows) != 1 {
		t.Fatal(result)
	}
	id := rows[0].(map[string]interface{})["id"].(float64)
	result = call("PUT", "/users/999/prices", fmt.Sprintf(`{"product_id":%d,"price_amount":"12.34"}`, product.ID))
	if result["status_code"] == float64(0) {
		t.Fatal("accepted nonexistent user")
	}
	result = call("DELETE", fmt.Sprintf("%s/%d", path, int(id)), "")
	if result["status_code"] != float64(0) {
		t.Fatal(result)
	}
	result = call("GET", path, "")
	if len(result["data"].([]interface{})) != 0 {
		t.Fatal("price not deleted")
	}
}
