package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

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
		AccountEmail: "buyer@example.com",
		CurrentPlan:  "free",
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
	if savedItem.PostPaymentAccountEmail != "buyer@example.com" || savedItem.PostPaymentCurrentPlan != "free" || savedItem.PostPaymentInfoSubmittedAt == nil {
		t.Fatalf("post-payment info was not saved: %+v", savedItem)
	}

	input.UserID = 8
	if _, err := svc.SubmitPostPaymentInfo(input); err != ErrOrderNotFound {
		t.Fatalf("another user must not update the order, got %v", err)
	}
}
