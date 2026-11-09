package models

import "time"

// OrderInvoice keeps invoice documents private; never expose FileData in JSON.
type OrderInvoice struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	OrderID         uint       `gorm:"uniqueIndex;not null" json:"order_id"`
	OrderNo         string     `gorm:"index;not null" json:"order_no"`
	UserID          uint       `gorm:"index;not null" json:"user_id"`
	Kind            string     `json:"kind"`
	Title           string     `json:"title"`
	TaxID           string     `json:"tax_id"`
	Email           string     `json:"email"`
	Note            string     `json:"note"`
	Amount          Money      `gorm:"type:decimal(20,2);not null" json:"amount"`
	Currency        string     `json:"currency"`
	Status          string     `gorm:"index;not null" json:"status"`
	RejectionReason string     `json:"rejection_reason"`
	ReviewedBy      uint       `json:"reviewed_by"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	FileData        []byte     `json:"-"`
	FileName        string     `json:"file_name"`
	UploadedBy      uint       `json:"uploaded_by"`
	MailStatus      string     `gorm:"index" json:"mail_status"`
	MailError       string     `json:"mail_error"`
	SendingAt       *time.Time `json:"sending_at"`
	SentAt          *time.Time `json:"sent_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
