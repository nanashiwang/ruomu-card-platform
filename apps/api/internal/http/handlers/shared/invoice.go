package shared

import (
	"errors"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InvoiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, service.ErrOrderNotFound):
		RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
	case errors.Is(err, service.ErrInvoiceInvalid), errors.Is(err, service.ErrInvoiceState), errors.Is(err, service.ErrInvoicePDF):
		RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
	case errors.Is(err, service.ErrInvoiceMail):
		RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
	default:
		RespondErrorWithMsg(c, response.CodeInternal, "发票操作失败，请稍后刷新重试", err)
	}
}
func InvoiceDocument(c *gin.Context, name string, data []byte) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename=\""+name+"\"")
	c.Data(200, "application/pdf", data)
}
