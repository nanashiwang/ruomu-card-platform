package service

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// SubmitPostPaymentInfoInput 是用户付款后补充人工处理资料的输入。
type SubmitPostPaymentInfoInput struct {
	Tenant       TenantContext
	OrderNo      string
	UserID       uint
	OrderItemID  uint
	AccountEmail string
	CurrentPlan  string
}

var allowedPostPaymentPlans = map[string]struct{}{
	"free":       {},
	"go":         {},
	"plus":       {},
	"pro":        {},
	"business":   {},
	"team":       {},
	"enterprise": {},
	"edu":        {},
	"other":      {},
}

// SubmitPostPaymentInfo 保存用户付款后的账号邮箱与当前套餐。
// 只有订单所有者可在已付款、待人工处理阶段提交或修改。
func (s *OrderService) SubmitPostPaymentInfo(input SubmitPostPaymentInfoInput) (*models.Order, error) {
	if input.UserID == 0 || input.OrderItemID == 0 || strings.TrimSpace(input.OrderNo) == "" {
		return nil, ErrPostPaymentInfoInvalid
	}

	email := strings.ToLower(strings.TrimSpace(input.AccountEmail))
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsedEmail.Address, email) || len(email) > 254 {
		return nil, ErrPostPaymentInfoInvalid
	}
	plan := strings.ToLower(strings.TrimSpace(input.CurrentPlan))
	if _, ok := allowedPostPaymentPlans[plan]; !ok {
		return nil, ErrPostPaymentInfoInvalid
	}

	order, err := s.GetOrderByUserOrderNoForTenant(input.Tenant, input.OrderNo, input.UserID)
	if err != nil {
		return nil, err
	}

	var target *models.OrderItem
	targetStatus := order.Status
	for i := range order.Items {
		if order.Items[i].ID == input.OrderItemID {
			target = &order.Items[i]
			break
		}
	}
	if target == nil {
		return nil, ErrPostPaymentInfoNotRequired
	}
	if target.OrderID != order.ID {
		for i := range order.Children {
			if order.Children[i].ID == target.OrderID {
				targetStatus = order.Children[i].Status
				break
			}
		}
	}
	if targetStatus != constants.OrderStatusPaid && targetStatus != constants.OrderStatusFulfilling {
		return nil, ErrOrderStatusInvalid
	}
	if !target.PostPaymentInfoRequired {
		return nil, ErrPostPaymentInfoNotRequired
	}

	now := time.Now()
	err = s.orderRepo.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.OrderItem{}).
			Where("id = ? AND order_id = ? AND post_payment_info_required = ?", target.ID, target.OrderID, true).
			Updates(map[string]interface{}{
				"post_payment_account_email":     email,
				"post_payment_current_plan":      plan,
				"post_payment_info_submitted_at": now,
				"updated_at":                     now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrPostPaymentInfoNotRequired
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPostPaymentInfoNotRequired) {
			return nil, err
		}
		return nil, ErrOrderUpdateFailed
	}

	return s.GetOrderByUserOrderNoForTenant(input.Tenant, input.OrderNo, input.UserID)
}
