package repository

import "gorm.io/gorm"

// BaseRepository 提供可嵌入的通用仓库方法。
type BaseRepository struct {
	db *gorm.DB
}

// Transaction 执行数据库事务。
func (b *BaseRepository) Transaction(fn func(tx *gorm.DB) error) error {
	if fn == nil {
		return nil
	}
	return b.db.Transaction(fn)
}
