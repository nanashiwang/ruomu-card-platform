package admin

import (
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) GetTranslationSettings(c *gin.Context) {
	setting, err := h.SettingService.GetTranslationSetting()
	if err != nil {
		response.Error(c, response.CodeInternal, err.Error())
		return
	}
	response.Success(c, setting.Public())
}

func (h *Handler) UpdateTranslationSettings(c *gin.Context) {
	var req struct {
		service.TranslationSetting
		ClearAPIKey bool `json:"clear_api_key"`
	}
	req.AutoOnSave = true
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, response.CodeBadRequest, "翻译配置格式错误")
		return
	}
	setting, err := h.SettingService.SaveTranslationSetting(req.TranslationSetting, req.ClearAPIKey)
	if err != nil {
		response.Error(c, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, setting.Public())
}

func (h *Handler) TranslateContent(c *gin.Context) {
	var req struct {
		Text         string `json:"text"`
		TargetLocale string `json:"target_locale"`
		UseCache     bool   `json:"use_cache"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 80*1024)
	if c.ShouldBindJSON(&req) != nil {
		response.Error(c, response.CodeBadRequest, "翻译请求格式错误或内容过长")
		return
	}
	text, err := h.SettingService.Translate(c.Request.Context(), req.Text, req.TargetLocale, req.UseCache)
	if err != nil {
		response.Error(c, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, gin.H{"text": text, "target_locale": req.TargetLocale})
}
