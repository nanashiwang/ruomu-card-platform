package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestNormalizePostPaymentContactOrderNoteLimit(t *testing.T) {
	email, normalized, ok := normalizePostPaymentContact(" Buyer@Example.com ", "   ")
	if !ok || email != "buyer@example.com" || normalized != "" {
		t.Fatalf("empty order note should be accepted and normalized, got email=%q note=%q ok=%v", email, normalized, ok)
	}

	accepted := strings.Repeat("测", postPaymentOrderNoteMaxLength)
	_, normalized, ok = normalizePostPaymentContact("buyer@example.com", accepted)
	if !ok || len([]rune(normalized)) != postPaymentOrderNoteMaxLength {
		t.Fatalf("%d-character order note should be accepted", postPaymentOrderNoteMaxLength)
	}

	rejected := strings.Repeat("测", postPaymentOrderNoteMaxLength+1)
	if _, _, ok := normalizePostPaymentContact("buyer@example.com", rejected); ok {
		t.Fatalf("order note longer than %d characters should be rejected", postPaymentOrderNoteMaxLength)
	}
}

func TestSubmitPostPaymentInfoRequiresPaidOwnedOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:post_payment_info_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Order{}, &models.OrderItem{}, &models.OrderRefundRecord{}, &models.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	order := &models.Order{
		OrderNo:        "POST-INFO-001",
		UserID:         7,
		Status:         constants.OrderStatusPendingPayment,
		Currency:       "CNY",
		OriginalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	item := models.OrderItem{
		ProductID:               1,
		SKUID:                   1,
		TitleJSON:               models.JSON{"zh-CN": "GPT Plus"},
		Quantity:                1,
		UnitPrice:               models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		TotalPrice:              models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		FulfillmentType:         constants.FulfillmentTypeManual,
		PostPaymentInfoRequired: true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	repo := repository.NewOrderRepository(db)
	if err := repo.Create(order, []models.OrderItem{item}); err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	var savedItem models.OrderItem
	if err := db.Where("order_id = ?", order.ID).First(&savedItem).Error; err != nil {
		t.Fatalf("load item failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{OrderRepo: repo})
	input := SubmitPostPaymentInfoInput{
		Tenant:       MainTenantContext("main.example.test"),
		OrderNo:      order.OrderNo,
		UserID:       order.UserID,
		OrderItemID:  savedItem.ID,
		ContactEmail: "buyer@example.com",
		CurrentPlan:  "free",
		OrderNote:    "请通过邮箱联系我处理订单",
	}
	if _, err := svc.SubmitPostPaymentInfo(input); err != ErrOrderStatusInvalid {
		t.Fatalf("pending order should reject submission, got %v", err)
	}

	if err := db.Model(order).Updates(map[string]interface{}{"status": constants.OrderStatusPaid, "paid_at": now}).Error; err != nil {
		t.Fatalf("mark order paid failed: %v", err)
	}
	if _, err := svc.SubmitPostPaymentInfo(input); err != nil {
		t.Fatalf("paid order should accept submission: %v", err)
	}
	if err := db.First(&savedItem, savedItem.ID).Error; err != nil {
		t.Fatalf("reload item failed: %v", err)
	}
	if savedItem.PostPaymentAccountEmail != "" || savedItem.PostPaymentContactEmail != "buyer@example.com" || savedItem.PostPaymentCurrentPlan != "free" || savedItem.PostPaymentOrderNote != "请通过邮箱联系我处理订单" || savedItem.PostPaymentInfoSubmittedAt == nil {
		t.Fatalf("post-payment info was not saved: %+v", savedItem)
	}

	input.OrderNote = "   "
	if _, err := svc.SubmitPostPaymentInfo(input); err != nil {
		t.Fatalf("empty order note should be accepted: %v", err)
	}
	if err := db.First(&savedItem, savedItem.ID).Error; err != nil {
		t.Fatalf("reload item with empty order note failed: %v", err)
	}
	if savedItem.PostPaymentOrderNote != "" {
		t.Fatalf("empty order note should be stored as an empty string, got %q", savedItem.PostPaymentOrderNote)
	}

	input.UserID = 8
	if _, err := svc.SubmitPostPaymentInfo(input); err != ErrOrderNotFound {
		t.Fatalf("another user must not update the order, got %v", err)
	}
}

func TestSubmitPostPaymentInfoUsesRealChildOrderID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:child_post_payment_info_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Order{}, &models.OrderItem{}, &models.OrderRefundRecord{}, &models.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	parent := &models.Order{
		OrderNo:        "POST-INFO-PARENT-001",
		UserID:         7,
		Status:         constants.OrderStatusPaid,
		Currency:       "CNY",
		OriginalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		PaidAt:         &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	repo := repository.NewOrderRepository(db)
	if err := repo.Create(parent, nil); err != nil {
		t.Fatalf("create parent order failed: %v", err)
	}

	child := &models.Order{
		OrderNo:        "POST-INFO-PARENT-001-01",
		ParentID:       &parent.ID,
		UserID:         parent.UserID,
		Status:         constants.OrderStatusPaid,
		Currency:       parent.Currency,
		OriginalAmount: parent.OriginalAmount,
		TotalAmount:    parent.TotalAmount,
		PaidAt:         &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	item := models.OrderItem{
		ProductID:               1,
		SKUID:                   1,
		TitleJSON:               models.JSON{"zh-CN": "GPT Plus"},
		Quantity:                1,
		UnitPrice:               models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		TotalPrice:              models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		FulfillmentType:         constants.FulfillmentTypeManual,
		PostPaymentInfoRequired: true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := repo.Create(child, []models.OrderItem{item}); err != nil {
		t.Fatalf("create child order failed: %v", err)
	}
	var savedItem models.OrderItem
	if err := db.Where("order_id = ?", child.ID).First(&savedItem).Error; err != nil {
		t.Fatalf("load child item failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{OrderRepo: repo})
	_, err = svc.SubmitPostPaymentInfo(SubmitPostPaymentInfoInput{
		Tenant:       MainTenantContext("main.example.test"),
		OrderNo:      parent.OrderNo,
		UserID:       parent.UserID,
		OrderItemID:  savedItem.ID,
		ContactEmail: "buyer@example.com",
		CurrentPlan:  "plus",
		OrderNote:    "父订单页面提交的资料",
	})
	if err != nil {
		t.Fatalf("parent order page should update the real child item: %v", err)
	}
	if err := db.First(&savedItem, savedItem.ID).Error; err != nil {
		t.Fatalf("reload child item failed: %v", err)
	}
	if savedItem.OrderID != child.ID || savedItem.PostPaymentContactEmail != "buyer@example.com" || savedItem.PostPaymentCurrentPlan != "plus" || savedItem.PostPaymentOrderNote != "父订单页面提交的资料" || savedItem.PostPaymentInfoSubmittedAt == nil {
		t.Fatalf("child post-payment info was not saved: %+v", savedItem)
	}
}

func TestSubmitGuestPostPaymentInfoRequiresGuestCredentials(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:guest_post_payment_info_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Order{}, &models.OrderItem{}, &models.OrderRefundRecord{}, &models.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	order := &models.Order{
		OrderNo: "GUEST-POST-INFO-001", GuestEmail: "guest@example.com", GuestPassword: "lookup-code",
		Status: constants.OrderStatusPaid, Currency: "CNY",
		OriginalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(169)), CreatedAt: now, UpdatedAt: now,
	}
	item := models.OrderItem{
		ProductID: 1, SKUID: 1, TitleJSON: models.JSON{"zh-CN": "GPT Plus"}, Quantity: 1,
		UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(169)), TotalPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(169)),
		FulfillmentType: constants.FulfillmentTypeManual, PostPaymentInfoRequired: true, CreatedAt: now, UpdatedAt: now,
	}
	repo := repository.NewOrderRepository(db)
	if err := repo.Create(order, []models.OrderItem{item}); err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	var savedItem models.OrderItem
	if err := db.Where("order_id = ?", order.ID).First(&savedItem).Error; err != nil {
		t.Fatalf("load item failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{OrderRepo: repo})
	input := SubmitGuestPostPaymentInfoInput{
		Tenant: MainTenantContext("main.example.test"), OrderNo: order.OrderNo,
		GuestEmail: order.GuestEmail, GuestPassword: order.GuestPassword, OrderItemID: savedItem.ID,
		ContactEmail: "contact@example.com", CurrentPlan: "free", OrderNote: "游客订单备注",
	}
	if _, err := svc.SubmitGuestPostPaymentInfo(input); err != nil {
		t.Fatalf("valid guest credentials should accept submission: %v", err)
	}
	input.GuestPassword = "wrong-code"
	if _, err := svc.SubmitGuestPostPaymentInfo(input); err != ErrGuestOrderNotFound {
		t.Fatalf("invalid guest credentials must be rejected, got %v", err)
	}
}
