package service

import (
	"bytes"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/telegramidentity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const InvoicePDFLimit = 5 * 1024 * 1024

var ErrInvoiceInvalid = errors.New("请检查抬头、税号、邮箱和备注")
var ErrInvoiceState = errors.New("申请状态已变化，请刷新；仅支持已付款、未退款的主订单")
var ErrInvoicePDF = errors.New("请上传不超过 5 MiB 的有效 PDF 发票")
var ErrInvoiceMail = errors.New("邮件发送失败或结果不确定，请检查邮件配置与收件邮箱后重试")

type InvoiceInput struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	TaxID string `json:"tax_id"`
	Email string `json:"email"`
	Note  string `json:"note"`
}

type InvoiceService struct {
	notify func(NotificationEnqueueInput) error
	db     *gorm.DB
	send   func(string, string, string, []byte) error
}

func NewInvoiceService(db *gorm.DB, email *EmailService) *InvoiceService {
	return &InvoiceService{db: db, send: email.SendInvoicePDF}
}

func (s *InvoiceService) SetNotificationService(notifications *NotificationService) {
	s.notify = notifications.Enqueue
}

func invoiceOrderEligible(o *models.Order) bool {
	if o.ParentID != nil || o.UserID == 0 || o.PaidAt == nil || !o.TotalAmount.IsPositive() || !o.RefundedAmount.IsZero() {
		return false
	}
	switch o.Status {
	case constants.OrderStatusPaid, constants.OrderStatusFulfilling, constants.OrderStatusPartiallyDelivered, constants.OrderStatusDelivered, constants.OrderStatusCompleted:
		return true
	}
	return false
}

// eligibleOrder also checks children: a partially refunded child can precede parent synchronization.
func (s *InvoiceService) eligibleOrder(db *gorm.DB, id uint) (*models.Order, error) {
	var o models.Order
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&o, id).Error; err != nil {
		return nil, err
	}
	if !invoiceOrderEligible(&o) {
		return nil, ErrInvoiceState
	}
	var count int64
	if err := db.Model(&models.Order{}).Where("parent_id = ? AND (refunded_amount > 0 OR status IN ?)", id, []string{constants.OrderStatusRefunded, constants.OrderStatusPartiallyRefunded, constants.OrderStatusCanceled}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrInvoiceState
	}
	return &o, nil
}

func (s *InvoiceService) Apply(orderID, userID uint, in InvoiceInput) (*models.OrderInvoice, error) {
	in.Title, in.TaxID, in.Email, in.Note = strings.TrimSpace(in.Title), strings.ToUpper(strings.TrimSpace(in.TaxID)), strings.TrimSpace(in.Email), strings.TrimSpace(in.Note)
	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != in.Email || strings.ContainsAny(in.Email, "\r\n") || len(in.Email) > 254 || telegramidentity.IsPlaceholderEmail(in.Email) || in.Title == "" || utf8.RuneCountInString(in.Title) > 200 || utf8.RuneCountInString(in.Note) > 1000 || (in.Kind != "personal" && in.Kind != "company") {
		return nil, ErrInvoiceInvalid
	}
	if in.Kind == "company" {
		if len(in.TaxID) < 15 || len(in.TaxID) > 20 || strings.IndexFunc(in.TaxID, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z') }) >= 0 {
			return nil, ErrInvoiceInvalid
		}
	} else {
		in.TaxID = ""
	}
	var notification NotificationEnqueueInput
	err = s.db.Transaction(func(tx *gorm.DB) error {
		o, e := s.eligibleOrder(tx, orderID)
		if e != nil {
			return e
		}
		if userID == 0 || o.UserID != userID {
			return gorm.ErrRecordNotFound
		}
		notification = NotificationEnqueueInput{EventType: constants.NotificationEventInvoiceRequested, BizType: "invoice", BizID: o.ID,
			// Each committed submission is a distinct event. Bypass time dedupe so retries
			// and rejected/resubmitted requests are not silently suppressed.
			Force: true, Data: models.JSON{"order_no": o.OrderNo, "amount": o.TotalAmount.String(), "currency": o.Currency}}
		record := models.OrderInvoice{OrderID: o.ID, OrderNo: o.OrderNo, UserID: o.UserID, Kind: in.Kind, Title: in.Title, TaxID: in.TaxID, Email: in.Email, Note: in.Note, Amount: o.TotalAmount, Currency: o.Currency, Status: "pending", MailStatus: "pending"}
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "order_id"}}, DoNothing: true}).Create(&record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			return nil
		}
		result = tx.Model(&models.OrderInvoice{}).Where("order_id = ? AND status = ?", orderID, "rejected").Updates(map[string]any{"kind": in.Kind, "title": in.Title, "tax_id": in.TaxID, "email": in.Email, "note": in.Note, "amount": o.TotalAmount, "currency": o.Currency, "status": "pending", "rejection_reason": "", "reviewed_by": 0, "reviewed_at": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvoiceState
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.notify != nil {
		if err := s.notify(notification); err != nil {
			logger.Warnw("invoice_notification_enqueue_failed", "order_id", orderID, "error", err)
		}
	}
	return s.ForOrder(orderID)
}
func (s *InvoiceService) ForOrder(orderID uint) (*models.OrderInvoice, error) {
	var item models.OrderInvoice
	err := s.db.Omit("file_data").Where("order_id = ?", orderID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}
func (s *InvoiceService) List(page int, status, orderNo string) ([]models.OrderInvoice, int64, error) {
	q := s.db.Model(&models.OrderInvoice{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if orderNo != "" {
		q = q.Where("order_no = ?", orderNo)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	items := []models.OrderInvoice{}
	err := q.Omit("file_data").Order("id DESC").Limit(20).Offset((page - 1) * 20).Find(&items).Error
	return items, count, err
}

// Recheck the order amount against the application snapshot before issuing/sending.
func (s *InvoiceService) checkSnapshot(tx *gorm.DB, item *models.OrderInvoice) error {
	order, err := s.eligibleOrder(tx, item.OrderID)
	if err != nil {
		return err
	}
	if !order.TotalAmount.Equal(item.Amount.Decimal) || order.Currency != item.Currency {
		return ErrInvoiceState
	}
	return nil
}

func (s *InvoiceService) Review(id, adminID uint, approve bool, reason string) error {
	reason = strings.TrimSpace(reason)
	if !approve && (reason == "" || utf8.RuneCountInString(reason) > 1000) {
		return ErrInvoiceInvalid
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var item models.OrderInvoice
		if err := tx.Omit("file_data").First(&item, id).Error; err != nil {
			return err
		}
		status := "rejected"
		if approve {
			if err := s.checkSnapshot(tx, &item); err != nil {
				return err
			}
			status = "approved"
			reason = ""
		}
		result := tx.Model(&models.OrderInvoice{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]any{"status": status, "rejection_reason": reason, "reviewed_by": adminID, "reviewed_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvoiceState
		}
		return nil
	})
}
func validInvoicePDF(data []byte) bool {
	return len(data) > 0 && len(data) <= InvoicePDFLimit && bytes.HasPrefix(data, []byte("%PDF-")) && bytes.Contains(data[max(0, len(data)-1024):], []byte("%%EOF"))
}
func (s *InvoiceService) Upload(id, adminID uint, data []byte) error {
	if !validInvoicePDF(data) {
		return ErrInvoicePDF
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var item models.OrderInvoice
		if err := tx.Omit("file_data").First(&item, id).Error; err != nil {
			return err
		}
		if err := s.checkSnapshot(tx, &item); err != nil {
			return err
		}
		result := tx.Model(&models.OrderInvoice{}).Where("id = ? AND status = ?", id, "approved").Updates(map[string]any{"status": "issued", "file_data": data, "file_name": fmt.Sprintf("invoice-%d.pdf", id), "uploaded_by": adminID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvoiceState
		}
		return nil
	})
}
func (s *InvoiceService) Document(id uint) (*models.OrderInvoice, error) {
	var item models.OrderInvoice
	err := s.db.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	if item.Status != "issued" || len(item.FileData) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}
func (s *InvoiceService) Send(id uint) error {
	var item models.OrderInvoice
	// Persist a lease before SMTP. A crashed process can be retried after ten minutes.
	now := time.Now().UTC().Truncate(time.Microsecond)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&item, id).Error; err != nil {
			return err
		}
		if item.Status != "issued" || len(item.FileData) == 0 {
			return ErrInvoiceState
		}
		if item.MailStatus == "sent" {
			return nil
		}
		if err := s.checkSnapshot(tx, &item); err != nil {
			return err
		}
		r := tx.Model(&models.OrderInvoice{}).Where("id = ? AND status = ? AND (mail_status IN ? OR (mail_status = ? AND sending_at < ?))", id, "issued", []string{"pending", "failed"}, "sending", now.Add(-10*time.Minute)).Updates(map[string]any{"mail_status": "sending", "sending_at": now, "mail_error": ""})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return ErrInvoiceState
		}
		return nil
	})
	if err != nil {
		return err
	}
	if item.MailStatus == "sent" {
		return nil
	}
	sendErr := s.send(item.Email, item.OrderNo, item.FileName, item.FileData)
	updates := map[string]any{"mail_status": "sent", "sent_at": time.Now().UTC(), "mail_error": ""}
	if sendErr != nil {
		updates = map[string]any{"mail_status": "failed", "mail_error": ErrInvoiceMail.Error()}
	}
	r := s.db.Model(&models.OrderInvoice{}).Where("id = ? AND mail_status = ? AND sending_at = ?", id, "sending", now).Updates(updates)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrInvoiceState
	}
	if sendErr != nil {
		return ErrInvoiceMail
	}
	return nil
}
