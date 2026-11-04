package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/models"
)

type translationTransportFunc func(*http.Request) (*http.Response, error)

func (f translationTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAutomaticTranslationCache(t *testing.T) {
	s := NewSettingService(newMockSettingRepo())
	setting, err := s.GetTranslationSetting()
	if err != nil || !setting.AutoOnSave {
		t.Fatal("default automatic mode", err)
	}
	setting.Enabled = true
	setting.APIKey = "private-key"
	if _, err = s.SaveTranslationSetting(setting, false); err != nil {
		t.Fatal(err)
	}
	old := translationHTTPClient
	defer func() { translationHTTPClient = old }()
	calls := 0
	fail := false
	translationHTTPClient = &http.Client{Transport: translationTransportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"choices":[{"message":{"content":"Translated"},"finish_reason":"stop"}]}`
		status := http.StatusOK
		if fail {
			status = http.StatusTooManyRequests
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	for i := 0; i < 2; i++ {
		out, err := s.Translate(context.Background(), "中文", "en-US", true)
		if err != nil || out != "Translated" {
			t.Fatal(out, err)
		}
	}
	if calls != 1 {
		t.Fatalf("unchanged source called model %d times", calls)
	}
	// A new service instance sees the persisted cache.
	if _, err = NewSettingService(s.repo).Translate(context.Background(), "中文", "en-US", true); err != nil || calls != 1 {
		t.Fatal("persistent cache", err, calls)
	}
	for _, item := range []struct{ text, target string }{{"改文", "en-US"}, {"中文", "zh-TW"}} {
		if _, err = s.Translate(context.Background(), item.text, item.target, true); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatal(calls)
	}
	setting.Model = "qwen-mt-plus"
	_, _ = s.SaveTranslationSetting(setting, false)
	if _, err = s.Translate(context.Background(), "中文", "en-US", true); err != nil || calls != 4 {
		t.Fatal(err, calls)
	}
	if _, err = s.Translate(context.Background(), "中文", "en-US"); err != nil || calls != 5 {
		t.Fatal("manual regeneration must bypass cache", err, calls)
	}
	fail = true
	if _, err = s.Translate(context.Background(), "失败原文", "en-US", true); err == nil {
		t.Fatal("expected provider error")
	}
	cached, err := s.cachedTranslation(translationCacheKey(setting, "失败原文", "en-US"))
	if err != nil || cached != "" {
		t.Fatal("failed response cached")
	}
	setting.Enabled = false
	_, _ = s.SaveTranslationSetting(setting, false)
	if _, err = s.Translate(context.Background(), "中文", "en-US", true); err == nil {
		t.Fatal("disabled service served cache")
	}
	setting.Enabled = true
	setting.AutoOnSave = false
	_, _ = s.SaveTranslationSetting(setting, false)
	if _, err = s.Translate(context.Background(), "中文", "en-US", true); err == nil {
		t.Fatal("disabled auto mode served cache")
	}
}

func TestTranslationCacheBoundsAndPrivacy(t *testing.T) {
	s := NewSettingService(newMockSettingRepo())
	entries := map[string]translationCacheEntry{}
	for i := 0; i < translationCacheMaxEntries; i++ {
		key := strings.Repeat("x", i+1)
		entries[key] = translationCacheEntry{Text: "old", UpdatedAt: int64(i)}
	}
	data, _ := json.Marshal(entries)
	var raw models.JSON
	_ = json.Unmarshal(data, &raw)
	_, _ = s.Update(TranslationCacheSettingKey, raw)
	if err := s.rememberTranslation("new", "new output"); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.readTranslationCache()
	if err != nil || len(loaded) != translationCacheMaxEntries || loaded["x"].Text != "" || loaded["new"].Text != "new output" {
		t.Fatal("eviction failed", err)
	}
	if err = s.rememberTranslation("large", strings.Repeat("z", translationCacheMaxBytes/2)); err != nil {
		t.Fatal(err)
	}
	if err = s.rememberTranslation("larger", strings.Repeat("y", translationCacheMaxBytes/2)); err != nil {
		t.Fatal(err)
	}
	loaded, err = s.readTranslationCache()
	data, _ = json.Marshal(loaded)
	if err != nil || len(data) > translationCacheMaxBytes {
		t.Fatal("size bound", len(data), err)
	}
	key := translationCacheKey(TranslationSetting{BaseURL: "https://example.com", Model: "qwen-mt-flash", APIKey: "private"}, "中文原文", "en-US")
	if len(key) != 64 || strings.Contains(key, "private") || strings.Contains(key, "中文") {
		t.Fatal("cache key must only contain a digest")
	}
}
