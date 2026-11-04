package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
)

type translationTestRepo struct{ value models.JSON }

func (r *translationTestRepo) GetByKey(key string) (*models.Setting, error) {
	return &models.Setting{Key: key, ValueJSON: r.value}, nil
}
func (r *translationTestRepo) Upsert(key string, value models.JSON) (*models.Setting, error) {
	r.value = value
	return &models.Setting{Key: key, ValueJSON: value}, nil
}

func TestTranslationSettingsCannotBypassDedicatedEndpoint(t *testing.T) {
	repo := &translationTestRepo{value: models.JSON{"api_key": "never-expose", "enabled": false, "base_url": "https://example.com/v1", "model": "qwen-mt-flash"}}
	h := &Handler{Container: &provider.Container{SettingService: service.NewSettingService(repo)}}
	for _, test := range []struct {
		method, path, body string
		handler            gin.HandlerFunc
	}{
		{"GET", "/settings?key=translation_config", "", h.GetSettings},
		{"GET", "/settings?key=translation_cache", "", h.GetSettings},
		{"PUT", "/settings", `{"key":"translation_cache","value":{"api_key":"bypass"}}`, h.UpdateSettings},
		{"PUT", "/settings", `{"key":"translation_config","value":{"api_key":"bypass"}}`, h.UpdateSettings},
		{"GET", "/settings/translation", "", h.GetTranslationSettings},
		{"PUT", "/settings/translation", `{"enabled":false,"base_url":"https://example.com/v1","model":"qwen-mt-flash","api_key":""}`, h.UpdateTranslationSettings},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		c.Request.Header.Set("Content-Type", "application/json")
		test.handler(c)
		if strings.Contains(w.Body.String(), "never-expose") || repo.value["api_key"] != "never-expose" {
			t.Fatal("secret leaked or overwritten", w.Body.String())
		}
		if strings.HasPrefix(test.path, "/settings?") || test.path == "/settings" {
			if !strings.Contains(w.Body.String(), `"status_code":403`) {
				t.Fatal(w.Body.String())
			}
		}
	}
}
