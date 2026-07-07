package models

import (
	"time"
)

// DownstreamOrderRef 下游订单引用表（B 侧记录来自下游站点的订单）
type DownstreamOrderRef struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	OrderID            uint       `gorm:"uniqueIndex;not null" json:"order_id"`
	ApiCredentialID    uint       `gorm:"index;not null" json:"api_credential_id"`
	DownstreamOrderNo  string     `gorm:"type:varchar(64);index" json:"downstream_order_no"`
	CallbackURL        string     `gorm:"type:varchar(500)" json:"callback_url"`
	TraceID            string     `gorm:"type:varchar(64);index" json:"trace_id"`
	CallbackStatus     string     `gorm:"type:varchar(20);not null;default:'pending'" json:"callback_status"`
	CallbackRetryCount int        `gorm:"not null;default:0" json:"callback_retry_count"`
	LastCallbackAt     *time.Time `json:"last_callback_at,omitempty"`
	CreatedAt          time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"index" json:"updated_at"`
}

// TableName 指定表名
func (DownstreamOrderRef) TableName() string {
	return "downstream_order_refs"
}
