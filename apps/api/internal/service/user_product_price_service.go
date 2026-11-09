package service

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/shopspring/decimal"
)

var userPriceAmountPattern = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,2})?$`)

var ErrUserProductPriceInvalid = errors.New("user_product_price_invalid")

type UserProductPriceService struct {
	repo     repository.UserProductPriceRepository
	users    repository.UserRepository
	products repository.ProductRepository
	skus     repository.ProductSKURepository
}

func NewUserProductPriceService(repo repository.UserProductPriceRepository, users repository.UserRepository, products repository.ProductRepository, skus repository.ProductSKURepository) *UserProductPriceService {
	return &UserProductPriceService{repo, users, products, skus}
}
func (s *UserProductPriceService) List(userID uint) ([]models.UserProductPrice, error) {
	return s.repo.ListByUser(userID)
}
func (s *UserProductPriceService) Save(userID, productID, skuID uint, amount string) error {
	if !userPriceAmountPattern.MatchString(amount) {
		return ErrUserProductPriceInvalid
	}
	price, err := decimal.NewFromString(amount)
	if err != nil || !price.GreaterThan(decimal.Zero) || !price.Equal(price.Round(2)) {
		return ErrUserProductPriceInvalid
	}
	user, err := s.users.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserProductPriceInvalid
	}
	product, err := s.products.GetByID(strconv.FormatUint(uint64(productID), 10))
	if err != nil {
		return err
	}
	if product == nil {
		return ErrUserProductPriceInvalid
	}
	if skuID > 0 {
		sku, err := s.skus.GetByID(skuID)
		if err != nil {
			return err
		}
		if sku == nil || sku.ProductID != productID {
			return ErrUserProductPriceInvalid
		}
	}
	return s.repo.Upsert(&models.UserProductPrice{UserID: userID, ProductID: productID, SKUID: skuID, PriceAmount: models.NewMoneyFromDecimal(price)})
}
func (s *UserProductPriceService) Delete(userID, id uint) error { return s.repo.Delete(userID, id) }
func (s *UserProductPriceService) Resolve(userID, productID, skuID uint) (*models.Money, error) {
	if s == nil || userID == 0 {
		return nil, nil
	}
	row, err := s.repo.Resolve(userID, productID, skuID)
	if err != nil || row == nil {
		return nil, err
	}
	return &row.PriceAmount, nil
}
