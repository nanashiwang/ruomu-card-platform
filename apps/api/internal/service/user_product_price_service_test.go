package service

import (
	"errors"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"testing"
)

func userPriceFixture(t *testing.T) (wholesaleOrderFixture, *UserProductPriceService) {
	t.Helper()
	rate, promo := decimal.NewFromInt(80), decimal.NewFromInt(30)
	f := setupWholesaleOrderFixture(t, t.Name(), models.WholesalePriceTiers{{MinQuantity: 2, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(60))}}, &promo, &rate)
	if err := f.db.AutoMigrate(&models.UserProductPrice{}, &models.CartItem{}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	s := NewUserProductPriceService(repository.NewUserProductPriceRepository(f.db), repository.NewUserRepository(f.db), repository.NewProductRepository(f.db), repository.NewProductSKURepository(f.db))
	f.svc.userProductPriceService = s
	return f, s
}
func TestUserProductPriceValidationIsolationAndFallback(t *testing.T) {
	f, s := userPriceFixture(t)
	for _, amount := range []string{"", "0", "-1", "1.001", "NaN", "9999999999999999999"} {
		if err := s.Save(f.user.ID, f.product.ID, 0, amount); !errors.Is(err, ErrUserProductPriceInvalid) {
			t.Fatalf("accepted %q: %v", amount, err)
		}
	}
	for _, ids := range [][3]uint{{999, f.product.ID, 0}, {f.user.ID, 999, 0}, {f.user.ID, f.product.ID, 999}} {
		if err := s.Save(ids[0], ids[1], ids[2], "10.00"); !errors.Is(err, ErrUserProductPriceInvalid) {
			t.Fatalf("accepted invalid target %v: %v", ids, err)
		}
	}
	other := models.Product{Slug: t.Name() + "-other", TitleJSON: models.JSON{"zh-CN": "另一商品"}, CategoryID: f.product.CategoryID}
	if err := f.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Save(f.user.ID, other.ID, f.sku.ID, "10"); !errors.Is(err, ErrUserProductPriceInvalid) {
		t.Fatal("accepted cross-product SKU", err)
	}
	for _, data := range []struct {
		sku   uint
		price string
	}{{0, "42.50"}, {f.sku.ID, "35"}, {f.sku.ID, "31.25"}} {
		if err := s.Save(f.user.ID, f.product.ID, data.sku, data.price); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.List(f.user.ID)
	if err != nil || len(rows) != 2 || rows[0].Product == nil || rows[1].SKU == nil {
		t.Fatalf("list/upsert: %+v %v", rows, err)
	}
	for _, userID := range []uint{0, f.user.ID + 1} {
		price, err := s.Resolve(userID, f.product.ID, f.sku.ID)
		if err != nil || price != nil {
			t.Fatalf("price leaked to %d", userID)
		}
	}
	price, err := s.Resolve(f.user.ID, f.product.ID, f.sku.ID)
	if err != nil || price.String() != "31.25" {
		t.Fatalf("SKU precedence: %v %v", price, err)
	}
	if err := s.Delete(f.user.ID+1, rows[1].ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross-user delete", err)
	}
	if err := s.Delete(f.user.ID, rows[1].ID); err != nil {
		t.Fatal(err)
	}
	price, err = s.Resolve(f.user.ID, f.product.ID, f.sku.ID)
	if err != nil || price.String() != "42.50" {
		t.Fatalf("product fallback: %v %v", price, err)
	}
	if err := s.Delete(f.user.ID, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	price, err = s.Resolve(f.user.ID, f.product.ID, f.sku.ID)
	if err != nil || price != nil {
		t.Fatal("delete fallback", err)
	}
	if err := s.Save(f.user.ID, f.product.ID, 0, "45"); err != nil {
		t.Fatal("recreate after deletion", err)
	}
}
func TestUserProductPriceOrderAndCartFixedPrice(t *testing.T) {
	f, s := userPriceFixture(t)
	for _, amount := range []string{"35.25", "125.00"} {
		if err := s.Save(f.user.ID, f.product.ID, 0, amount); err != nil {
			t.Fatal(err)
		}
		result, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 3}}})
		if err != nil {
			t.Fatal(err)
		}
		if result.OrderItems[0].UnitPrice.String() != amount || !result.TotalAmount.Equal(decimal.RequireFromString(amount).Mul(decimal.NewFromInt(3))) {
			t.Fatalf("incorrect fixed price: %+v", result)
		}
		preview, err := f.svc.PreviewOrder(CreateOrderInput{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 3}}})
		if err != nil || preview.Items[0].UnitPrice.String() != amount || !preview.TotalAmount.Decimal.Equal(result.TotalAmount) {
			t.Fatal("preview differs from order pricing", err)
		}
		if !result.PromotionDiscountAmount.IsZero() || !result.WholesaleDiscountAmount.IsZero() || !result.MemberDiscountAmount.IsZero() {
			t.Fatal("discounts stacked")
		}
	}
	// No membership is required for an individual price.
	if err := f.db.Model(&models.User{}).Where("id = ?", f.user.ID).Update("member_level_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	result, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1}}})
	if err != nil || result.OrderItems[0].UnitPrice.String() != "125.00" {
		t.Fatal("no-level user", err)
	}
	cart := NewCartService(repository.NewCartRepository(f.db), repository.NewProductRepository(f.db), repository.NewProductSKURepository(f.db), repository.NewPromotionRepository(f.db), nil)
	cart.SetUserProductPriceService(s)
	if err := f.db.Create(&models.CartItem{UserID: f.user.ID, ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 3}).Error; err != nil {
		t.Fatal(err)
	}
	items, err := cart.ListByUser(f.user.ID)
	if err != nil || len(items) != 1 || items[0].UnitPrice.String() != "125.00" {
		t.Fatalf("cart: %+v %v", items, err)
	}
	items, err = cart.ListByUserWithPricing(f.user.ID, false)
	if err != nil || len(items) != 1 || items[0].UnitPrice.String() != "70.00" {
		t.Fatalf("reseller cart applied user price: %+v %v", items, err)
	}
	// Query failures must never silently charge the normal price.
	if err := f.db.Migrator().DropTable(&models.UserProductPrice{}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1}}}); err == nil {
		t.Fatal("failed open on missing price storage")
	}
}
func TestUserProductPriceCouponAndDeleteRestorePricing(t *testing.T) {
	f, s := userPriceFixture(t)
	if err := s.Save(f.user.ID, f.product.ID, 0, "40"); err != nil {
		t.Fatal(err)
	}
	coupon := createWholesaleOrderCouponFixture(t, f.db, []uint{f.product.ID}, true)
	result, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, CouponCode: coupon.Code, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 3}}})
	if err != nil || result.TotalAmount.String() != "108" {
		t.Fatalf("coupon after user price: %+v %v", result, err)
	}
	rows, _ := s.List(f.user.ID)
	if err := s.Delete(f.user.ID, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	result, err = f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 3}}})
	if err != nil || result.OrderItems[0].UnitPrice.String() != "48.00" {
		t.Fatal("normal wholesale and membership not restored", err)
	}
}

func TestUserProductPriceGuestAndResellerOrderBoundaries(t *testing.T) {
	f, s := userPriceFixture(t)
	if err := s.Save(f.user.ID, f.product.ID, 0, "125"); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&models.Product{}).Where("id = ?", f.product.ID).Update("purchase_type", "guest").Error; err != nil {
		t.Fatal(err)
	}
	guest, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, IsGuest: true, GuestEmail: "guest@example.test", GuestPassword: "test-password", Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1}}})
	if err != nil || guest.OrderItems[0].UnitPrice.String() == "125.00" {
		t.Fatal("guest received user price", err)
	}
	reseller, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Tenant: ResellerTenantContext("shop.example", 9, 99, "shop.example"), Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1}}})
	if err != nil || reseller.OrderItems[0].UnitPrice.String() != "100.00" {
		t.Fatal("reseller base contaminated by main-store price", err)
	}
}
