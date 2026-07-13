package models

import (
	"time"

	"gorm.io/gorm"
)

// OrderItem 订单项表
type OrderItem struct {
	ID                           uint           `gorm:"primarykey" json:"id"`                                                   // 主键
	OrderID                      uint           `gorm:"index;not null" json:"order_id"`                                         // 订单ID
	ProductID                    uint           `gorm:"index;not null" json:"product_id"`                                       // 商品ID
	SKUID                        uint           `gorm:"column:sku_id;index;not null;default:0" json:"sku_id"`                   // SKU ID
	TitleJSON                    JSON           `gorm:"type:json;not null" json:"title"`                                        // 商品标题快照
	SKUSnapshotJSON              JSON           `gorm:"type:json" json:"sku_snapshot"`                                          // SKU 快照（编码/规格）
	Tags                         StringArray    `gorm:"type:json" json:"tags"`                                                  // 标签快照
	OriginalUnitPrice            Money          `gorm:"type:decimal(20,2);not null;default:0" json:"original_unit_price"`       // 原始单价快照
	UnitPrice                    Money          `gorm:"type:decimal(20,2);not null;default:0" json:"unit_price"`                // 单价
	CostPrice                    Money          `gorm:"type:decimal(20,2);not null;default:0" json:"cost_price"`                // 成本价快照
	Quantity                     int            `gorm:"not null" json:"quantity"`                                               // 数量
	OriginalTotalPrice           Money          `gorm:"type:decimal(20,2);not null;default:0" json:"original_total_price"`      // 原始小计快照
	TotalPrice                   Money          `gorm:"type:decimal(20,2);not null;default:0" json:"total_price"`               // 小计
	CouponDiscount               Money          `gorm:"type:decimal(20,2);not null;default:0" json:"coupon_discount_amount"`    // 优惠券分摊金额
	MemberDiscount               Money          `gorm:"type:decimal(20,2);not null;default:0" json:"member_discount_amount"`    // 会员优惠分摊金额
	PromotionDiscount            Money          `gorm:"type:decimal(20,2);not null;default:0" json:"promotion_discount_amount"` // 活动价分摊金额
	WholesaleDiscount            Money          `gorm:"type:decimal(20,2);not null;default:0" json:"wholesale_discount_amount"` // 批发价分摊金额
	PromotionID                  *uint          `gorm:"index" json:"promotion_id,omitempty"`                                    // 活动价ID
	PromotionName                string         `gorm:"-" json:"promotion_name,omitempty"`                                      // 活动价名称
	FulfillmentType              string         `gorm:"not null" json:"fulfillment_type"`                                       // 交付类型
	ManualFormSchemaSnapshotJSON JSON           `gorm:"type:json" json:"manual_form_schema_snapshot"`                           // 人工交付表单 schema 快照
	ManualFormSubmissionJSON     JSON           `gorm:"type:json" json:"manual_form_submission"`                                // 人工交付表单提交值
	PostPaymentInfoRequired      bool           `gorm:"not null;default:false" json:"post_payment_info_required"`               // 支付后补充资料要求快照
	PostPaymentAccountEmail      string         `gorm:"type:varchar(254)" json:"-"`                                             // 兼容旧数据，不再收集或返回
	PostPaymentContactEmail      string         `gorm:"type:varchar(254)" json:"post_payment_contact_email"`                    // 订单沟通联系邮箱（不是业务账号）
	PostPaymentCurrentPlan       string         `gorm:"type:varchar(32)" json:"post_payment_current_plan"`                      // 用户提交的当前套餐
	PostPaymentOrderNote         string         `gorm:"type:text" json:"post_payment_order_note"`                              // 用户提交的订单备注
	PostPaymentInfoSubmittedAt   *time.Time     `json:"post_payment_info_submitted_at,omitempty"`                               // 资料最后提交时间
	InstructionsJSON             JSON           `gorm:"type:json" json:"instructions"`                                          // 交付后使用说明快照（多语言）
	CreatedAt                    time.Time      `gorm:"index" json:"created_at"`                                                // 创建时间
	UpdatedAt                    time.Time      `gorm:"index" json:"updated_at"`                                                // 更新时间
	DeletedAt                    gorm.DeletedAt `gorm:"index" json:"-"`                                                         // 软删除时间
}

// TableName 指定表名
func (OrderItem) TableName() string {
	return "order_items"
}
