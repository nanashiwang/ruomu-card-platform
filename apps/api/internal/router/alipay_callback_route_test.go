package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Exercise the real router and middleware, not a hand-registered handler. An
// unknown synthetic trade must reach Alipay and return fail without authentication.
func TestProductionAlipayCallbackRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logs, restore := captureLogs(t)
	defer restore()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}, &models.Payment{}, &models.PaymentChannel{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	settings := service.NewSettingService(repository.NewSettingRepository(db))
	settings.InvalidateCallbackRoutesCache()
	t.Cleanup(settings.InvalidateCallbackRoutesCache)
	cfg := &config.Config{Server: config.ServerConfig{Mode: gin.TestMode}}
	container := &provider.Container{
		Config:             cfg,
		SettingService:     settings,
		PaymentRepo:        repository.NewPaymentRepository(db),
		PaymentChannelRepo: repository.NewPaymentChannelRepository(db),
	}
	router := SetupRouter(cfg, container)
	form := url.Values{
		"notify_id": {"route-test"}, "notify_type": {"trade_status_sync"},
		"out_trade_no": {"RUOMU-NONEXISTENT"}, "sign": {"invalid+signature/="},
		"trade_status": {"TRADE_SUCCESS"}, "total_amount": {"0.01"},
	}
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			target := "/api/v1/payments/callback"
			var req *http.Request
			if method == http.MethodPost {
				req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(method, target+"?"+form.Encode(), nil)
			}
			req.Host = "api.ruomu.test"
			req.Header.Set("X-Forwarded-Proto", "https")
			result := httptest.NewRecorder()
			router.ServeHTTP(result, req)
			require.Equal(t, http.StatusOK, result.Code)
			require.Equal(t, "fail", result.Body.String())
			require.Contains(t, result.Header().Get("Content-Type"), "text/plain")
			require.Empty(t, result.Header().Get("Location"))
		})
	}
	require.Equal(t, 2, logs.FilterMessage("alipay_callback_payment_not_found").Len())
	require.Equal(t, 0, logs.FilterMessage("panic_recovered").Len())

	// Existing installations can hide the default route via a database setting.
	// Keep this failure mode visible to the production routing probe.
	require.NoError(t, db.Create(&models.Setting{
		Key:       constants.SettingKeyCallbackRoutesConfig,
		ValueJSON: models.JSON{constants.SettingFieldPaymentCallback: "/api/custom/alipay-notify"},
	}).Error)
	settings.InvalidateCallbackRoutesCache()
	for path, status := range map[string]int{
		"/api/v1/payments/callback": http.StatusNotFound,
		"/api/custom/alipay-notify": http.StatusOK,
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		result := httptest.NewRecorder()
		router.ServeHTTP(result, req)
		require.Equal(t, status, result.Code)
		if status == http.StatusOK {
			require.Equal(t, "fail", result.Body.String())
		}
	}
}
