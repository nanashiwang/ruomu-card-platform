package repository

import (
	"github.com/dujiao-next/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserProductPriceRepository interface {
	ListByUser(uint) ([]models.UserProductPrice, error)
	Resolve(uint, uint, uint) (*models.UserProductPrice, error)
	Upsert(*models.UserProductPrice) error
	Delete(uint, uint) error
}
type GormUserProductPriceRepository struct{ db *gorm.DB }

func NewUserProductPriceRepository(db *gorm.DB) *GormUserProductPriceRepository {
	return &GormUserProductPriceRepository{db}
}
func (r *GormUserProductPriceRepository) ListByUser(userID uint) ([]models.UserProductPrice, error) {
	rows := []models.UserProductPrice{}
	err := r.db.Preload("Product").Preload("SKU").Where("user_id = ?", userID).Order("product_id, sku_id").Find(&rows).Error
	return rows, err
}
func (r *GormUserProductPriceRepository) Resolve(userID, productID, skuID uint) (*models.UserProductPrice, error) {
	var rows []models.UserProductPrice
	err := r.db.Where("user_id = ? AND product_id = ? AND sku_id IN ?", userID, productID, []uint{0, skuID}).Order("sku_id DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}
func (r *GormUserProductPriceRepository) Upsert(row *models.UserProductPrice) error {
	return r.db.Omit(clause.Associations).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "product_id"}, {Name: "sku_id"}}, DoUpdates: clause.AssignmentColumns([]string{"price_amount", "updated_at"})}).Create(row).Error
}
func (r *GormUserProductPriceRepository) Delete(userID, id uint) error {
	result := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&models.UserProductPrice{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
