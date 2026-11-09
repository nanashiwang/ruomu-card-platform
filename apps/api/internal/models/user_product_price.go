package models

import "time"

// UserProductPrice 是用户在主站的固定单价；SKUID=0 适用于商品所有规格。
type UserProductPrice struct {
	ID          uint        `gorm:"primarykey" json:"id"`
	UserID      uint        `gorm:"uniqueIndex:idx_user_product_price;not null" json:"user_id"`
	ProductID   uint        `gorm:"uniqueIndex:idx_user_product_price;not null" json:"product_id"`
	SKUID       uint        `gorm:"column:sku_id;uniqueIndex:idx_user_product_price;not null;default:0" json:"sku_id"`
	PriceAmount Money       `gorm:"type:decimal(20,2);not null" json:"price_amount"`
	Product     *Product    `gorm:"-:migration" json:"product,omitempty"`
	SKU         *ProductSKU `gorm:"foreignKey:SKUID;-:migration" json:"sku,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
