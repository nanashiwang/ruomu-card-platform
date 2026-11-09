package public

import (
	"github.com/dujiao-next/internal/dto"
	"github.com/gin-gonic/gin"
)

func setUserProductCacheHeaders(c *gin.Context) {
	if c.GetUint("user_id") > 0 {
		c.Header("Cache-Control", "private, no-store")
		c.Header("Vary", "Authorization")
	}
}

// Only authenticated routes populate user_id. Public callers cannot select a pricing identity.
func (h *Handler) applyUserProductPrices(c *gin.Context, product *dto.ProductResp, displaySKUID uint) error {
	userID := c.GetUint("user_id")
	if userID == 0 || isResellerTenant(tenantFromRequest(c)) || h.UserProductPriceService == nil {
		return nil
	}
	setUserProductCacheHeaders(c)
	for i := range product.SKUs {
		sku := &product.SKUs[i]
		if !sku.IsActive {
			continue
		}
		price, err := h.UserProductPriceService.Resolve(userID, product.ID, sku.ID)
		if err != nil {
			return err
		}
		if price == nil {
			continue
		}
		sku.PriceAmount = *price
		sku.UserPrice = true
		sku.PromotionPriceAmount = nil
		sku.MemberPriceAmount = nil
		if sku.ID == displaySKUID {
			product.PriceAmount = *price
			product.UserPrice = true
			product.PromotionID = nil
			product.PromotionName = ""
			product.PromotionType = ""
			product.PromotionPriceAmount = nil
		}
	}
	return nil
}
