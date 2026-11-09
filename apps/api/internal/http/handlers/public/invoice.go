package public

import (
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

func (h *Handler) invoiceOrder(c *gin.Context) *models.Order {
	uid, ok := shared.GetUserID(c)
	if !ok {
		return nil
	}
	order, err := h.OrderService.GetOrderByUserOrderNoForTenant(tenantFromRequest(c), c.Param("order_no"), uid)
	if err != nil {
		shared.InvoiceError(c, err)
		return nil
	}
	return order
}
func (h *Handler) GetInvoice(c *gin.Context) {
	order := h.invoiceOrder(c)
	if order == nil {
		return
	}
	item, err := h.InvoiceService.ForOrder(order.ID)
	if err != nil {
		shared.InvoiceError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, item)
}
func (h *Handler) ApplyInvoice(c *gin.Context) {
	order := h.invoiceOrder(c)
	if order == nil {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var in service.InvoiceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	item, err := h.InvoiceService.Apply(order.ID, order.UserID, in)
	if err != nil {
		shared.InvoiceError(c, err)
		return
	}
	response.Success(c, item)
}
func (h *Handler) DownloadInvoice(c *gin.Context) {
	order := h.invoiceOrder(c)
	if order == nil {
		return
	}
	item, err := h.InvoiceService.ForOrder(order.ID)
	if err != nil {
		shared.InvoiceError(c, err)
		return
	}
	if item == nil {
		shared.InvoiceError(c, gorm.ErrRecordNotFound)
		return
	}
	doc, err := h.InvoiceService.Document(item.ID)
	if err != nil {
		shared.InvoiceError(c, err)
		return
	}
	shared.InvoiceDocument(c, doc.FileName, doc.FileData)
}
