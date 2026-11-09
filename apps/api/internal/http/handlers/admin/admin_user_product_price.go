package admin

import (
	"errors"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) userPriceTarget(c *gin.Context) (uint, bool) {
	id, err := shared.ParseQueryUint(c.Param("id"), false)
	if err != nil || id == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", err)
		return 0, false
	}
	user, err := h.UserRepo.GetByID(id)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return 0, false
	}
	if user == nil {
		shared.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return 0, false
	}
	return id, true
}
func (h *Handler) GetUserProductPrices(c *gin.Context) {
	id, ok := h.userPriceTarget(c)
	if !ok {
		return
	}
	rows, err := h.UserProductPriceService.List(id)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return
	}
	response.Success(c, rows)
}
func (h *Handler) SetUserProductPrice(c *gin.Context) {
	id, ok := h.userPriceTarget(c)
	if !ok {
		return
	}
	var req struct {
		ProductID   uint   `json:"product_id" binding:"required"`
		SKUID       uint   `json:"sku_id"`
		PriceAmount string `json:"price_amount" binding:"required,max=24"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.UserProductPriceService.Save(id, req.ProductID, req.SKUID, req.PriceAmount); err != nil {
		if errors.Is(err, service.ErrUserProductPriceInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.user_price_invalid", nil)
		} else {
			shared.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, nil)
}
func (h *Handler) DeleteUserProductPrice(c *gin.Context) {
	id, ok := h.userPriceTarget(c)
	if !ok {
		return
	}
	priceID, err := shared.ParseQueryUint(c.Param("price_id"), false)
	if err != nil || priceID == 0 {
		shared.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err = h.UserProductPriceService.Delete(id, priceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.user_price_not_found", nil)
		} else {
			shared.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	response.Success(c, nil)
}
