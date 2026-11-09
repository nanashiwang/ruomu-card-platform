package admin

import (
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func invoiceID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		shared.InvoiceError(c, service.ErrInvoiceInvalid)
		return 0, false
	}
	return uint(id), true
}
func (h *Handler) ListInvoices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	items, total, err := h.InvoiceService.List(page, strings.TrimSpace(c.Query("status")), strings.TrimSpace(c.Query("order_no")))
	if err != nil {
		shared.InvoiceError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": 20})
}
func (h *Handler) ReviewInvoice(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	aid, ok := shared.GetAdminID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	var in struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		shared.RespondBindError(c, err)
		return
	}
	if in.Decision != "approve" && in.Decision != "reject" {
		shared.InvoiceError(c, service.ErrInvoiceInvalid)
		return
	}
	if err := h.InvoiceService.Review(id, aid, in.Decision == "approve", in.Reason); err != nil {
		shared.InvoiceError(c, err)
		return
	}
	response.Success(c, nil)
}
func (h *Handler) UploadInvoice(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	aid, ok := shared.GetAdminID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.InvoicePDFLimit+64*1024)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		shared.InvoiceError(c, service.ErrInvoicePDF)
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" {
		shared.InvoiceError(c, service.ErrInvoicePDF)
		return
	}
	defer part.Close()
	data, err := io.ReadAll(io.LimitReader(part, service.InvoicePDFLimit+1))
	if err != nil {
		shared.InvoiceError(c, service.ErrInvoicePDF)
		return
	}
	if err = h.InvoiceService.Upload(id, aid, data); err != nil {
		shared.InvoiceError(c, err)
		return
	}
	response.Success(c, nil)
}
func (h *Handler) DownloadInvoice(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	doc, err := h.InvoiceService.Document(id)
	if err != nil {
		shared.InvoiceError(c, err)
		return
	}
	shared.InvoiceDocument(c, doc.FileName, doc.FileData)
}
func (h *Handler) SendInvoice(c *gin.Context) {
	id, ok := invoiceID(c)
	if !ok {
		return
	}
	if err := h.InvoiceService.Send(id); err != nil {
		shared.InvoiceError(c, err)
		return
	}
	response.Success(c, nil)
}
