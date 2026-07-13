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
	ContactEmail string
	CurrentPlan  string
	OrderNote    string
}

// SubmitGuestPostPaymentInfoInput 是游客凭订单查询凭证补充资料的输入。
type SubmitGuestPostPaymentInfoInput struct {
	Tenant        TenantContext
	OrderNo       string
	GuestEmail    string
	GuestPassword string
	OrderItemID   uint
	ContactEmail  string
	CurrentPlan   string
	OrderNote     string
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

// SubmitPostPaymentInfo 保存用户付款后的联系邮箱、当前套餐和订单备注。
// 只有订单所有者可在已付款、待人工处理阶段提交或修改。
func (s *OrderService) SubmitPostPaymentInfo(input SubmitPostPaymentInfoInput) (*models.Order, error) {
	if input.UserID == 0 || input.OrderItemID == 0 || strings.TrimSpace(input.OrderNo) == "" {
		return nil, ErrPostPaymentInfoInvalid
	}
	contactEmail, orderNote, ok := normalizePostPaymentContact(input.ContactEmail, input.OrderNote)
	if !ok {
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
	return s.savePostPaymentInfo(order, input.OrderItemID, contactEmail, plan, orderNote, func() (*models.Order, error) {
		return s.GetOrderByUserOrderNoForTenant(input.Tenant, input.OrderNo, input.UserID)
	})
}

// SubmitGuestPostPaymentInfo 允许游客使用下单邮箱和订单查询凭证提交资料。
func (s *OrderService) SubmitGuestPostPaymentInfo(input SubmitGuestPostPaymentInfoInput) (*models.Order, error) {
	if input.OrderItemID == 0 || strings.TrimSpace(input.OrderNo) == "" || strings.TrimSpace(input.GuestEmail) == "" || strings.TrimSpace(input.GuestPassword) == "" {
		return nil, ErrPostPaymentInfoInvalid
	}
	contactEmail, orderNote, ok := normalizePostPaymentContact(input.ContactEmail, input.OrderNote)
	if !ok {
		return nil, ErrPostPaymentInfoInvalid
	}
	plan := strings.ToLower(strings.TrimSpace(input.CurrentPlan))
	if _, ok := allowedPostPaymentPlans[plan]; !ok {
		return nil, ErrPostPaymentInfoInvalid
	}

	order, err := s.GetOrderByGuestOrderNoForTenant(input.Tenant, input.OrderNo, input.GuestEmail, input.GuestPassword)
	if err != nil {
		return nil, err
	}
	return s.savePostPaymentInfo(order, input.OrderItemID, contactEmail, plan, orderNote, func() (*models.Order, error) {
		return s.GetOrderByGuestOrderNoForTenant(input.Tenant, input.OrderNo, input.GuestEmail, input.GuestPassword)
	})
}

func normalizePostPaymentContact(rawEmail, rawNote string) (string, string, bool) {
	email := strings.ToLower(strings.TrimSpace(rawEmail))
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsedEmail.Address, email) || len(email) > 254 {
		return "", "", false
	}
	note := strings.TrimSpace(rawNote)
	if note == "" || len([]rune(note)) > 1000 {
		return "", "", false
	}
	return email, note, true
}

func (s *OrderService) savePostPaymentInfo(order *models.Order, orderItemID uint, contactEmail, plan, orderNote string, reload func() (*models.Order, error)) (*models.Order, error) {
	var target *models.OrderItem
	targetOrderID := order.ID
	targetStatus := order.Status
	// 父订单详情会把子订单商品复制到 order.Items 供前端兼容展示，并把副本的
	// OrderID 改成父订单 ID。保存资料时必须优先定位真实的子订单项，不能使用副本。
	for i := range order.Children {
		for j := range order.Children[i].Items {
			if order.Children[i].Items[j].ID == orderItemID {
				target = &order.Children[i].Items[j]
				targetOrderID = order.Children[i].ID
				targetStatus = order.Children[i].Status
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		for i := range order.Items {
			if order.Items[i].ID == orderItemID {
				target = &order.Items[i]
				break
			}
		}
	}
	if target == nil {
		return nil, ErrPostPaymentInfoNotRequired
	}
	if targetStatus != constants.OrderStatusPaid && targetStatus != constants.OrderStatusFulfilling {
		return nil, ErrOrderStatusInvalid
	}
	if !target.PostPaymentInfoRequired {
		return nil, ErrPostPaymentInfoNotRequired
	}

	now := time.Now()
	err := s.orderRepo.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.OrderItem{}).
			Where("id = ? AND order_id = ? AND post_payment_info_required = ?", target.ID, targetOrderID, true).
			Updates(map[string]interface{}{
				"post_payment_account_email":     "",
				"post_payment_contact_email":     contactEmail,
				"post_payment_current_plan":      plan,
				"post_payment_order_note":        orderNote,
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

	return reload()
}
