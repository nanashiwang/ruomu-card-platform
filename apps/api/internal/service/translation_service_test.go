package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTranslationSettingsSecretLifecycle(t *testing.T) {
	s := NewSettingService(newMockSettingRepo())
	cfg, err := s.GetTranslationSetting()
	if err != nil || cfg.Enabled || cfg.Model != "qwen-mt-flash" {
		t.Fatal(cfg, err)
	}
	cfg.Enabled = true
	cfg.APIKey = "test-secret"
	cfg, err = s.SaveTranslationSetting(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(cfg.Public())
	if strings.Contains(string(data), "test-secret") || cfg.Public()["has_api_key"] != true {
		t.Fatal("secret exposure")
	}
	cfg.APIKey = ""
	cfg, err = s.SaveTranslationSetting(cfg, false)
	if err != nil || cfg.APIKey != "test-secret" {
		t.Fatal("blank must preserve key", err)
	}
	if _, err = s.SaveTranslationSetting(cfg, true); err == nil {
		t.Fatal("cannot clear enabled key")
	}
	cfg.Enabled = false
	cfg, err = s.SaveTranslationSetting(cfg, true)
	if err != nil || cfg.APIKey != "" {
		t.Fatal("clear failed", err)
	}
	for _, endpoint := range []string{"http://example.com/v1", "https://user:secret@example.com/v1", "https://127.0.0.1/v1", "https://example.com/v1?key=secret", "https://example.com/v1/chat/completions", "https://example.com:8000/v1"} {
		cfg.BaseURL = endpoint
		if _, err = s.SaveTranslationSetting(cfg, false); err == nil {
			t.Fatalf("unsafe endpoint accepted: %s", endpoint)
		}
	}
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::1", "fe80::1", "fc00::1", "0.0.0.0"} {
		if translationPublicIP(net.ParseIP(address)) {
			t.Fatal(address)
		}
	}
}

func TestTranslationProtocolAndStructure(t *testing.T) {
	for _, target := range []string{"en-US", "zh-TW"} {
		t.Run(target, func(t *testing.T) {
			source := "<p>你好 {{order_no}} [说明](https://example.com/a) `code`</p>"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-secret" {
					t.Error("request endpoint/auth")
				}
				var payload struct {
					Model              string                           `json:"model"`
					Messages           []struct{ Role, Content string } `json:"messages"`
					TranslationOptions map[string]string                `json:"translation_options"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Model != "qwen-mt-flash" || len(payload.Messages) != 1 || payload.Messages[0].Role != "user" || payload.TranslationOptions["source_lang"] != "Chinese" || payload.TranslationOptions["target_lang"] != translationLanguages[target] {
					t.Error("incorrect Qwen protocol")
				}
				out := strings.ReplaceAll(strings.ReplaceAll(payload.Messages[0].Content, "你好", "Hello"), "说明", "Guide")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]string{"content": out}, "finish_reason": "stop"}}})
			}))
			defer server.Close()
			out, err := requestTranslation(context.Background(), server.Client(), TranslationSetting{BaseURL: server.URL + "/v1", Model: "qwen-mt-flash", APIKey: "test-secret"}, source, target)
			if err != nil || out != "<p>Hello {{order_no}} [Guide](https://example.com/a) `code`</p>" {
				t.Fatal(out, err)
			}
		})
	}
}

func TestTranslationRejectsInvalidOutput(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"upstream", 401, `{"error":"secret-from-upstream"}`},
		{"invalid", 200, `not json`},
		{"truncated", 200, `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`},
		{"empty", 200, `{"choices":[{"message":{"content":" "},"finish_reason":"stop"}]}`},
		{"lost placeholder", 200, `{"choices":[{"message":{"content":"Hello"},"finish_reason":"stop"}]}`},
		{"new html", 200, `{"choices":[{"message":{"content":"<script>bad</script> RUOMUKEEP0TOKEN"},"finish_reason":"stop"}]}`},
		{"oversized", 200, strings.Repeat("x", 256*1024+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := requestTranslation(context.Background(), server.Client(), TranslationSetting{BaseURL: server.URL}, "你好 {{order_no}}", "en-US")
			if err == nil || strings.Contains(err.Error(), "secret-from-upstream") {
				t.Fatal(err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := requestTranslation(ctx, &http.Client{Timeout: time.Millisecond}, TranslationSetting{BaseURL: "https://example.invalid"}, "你好", "en-US")
	if err == nil {
		t.Fatal("expected canceled request")
	}
	if _, err = restoreTranslation("RUOMUKEEP1TOKEN RUOMUKEEP0TOKEN", []string{"<p>", "</p>"}); err == nil {
		t.Fatal("reordered tags accepted")
	}
	if _, _, err = protectTranslation("RUOMUKEEP0TOKEN"); err == nil {
		t.Fatal("marker collision")
	}
}

func TestTranslationValidationDoesNotCallProvider(t *testing.T) {
	s := NewSettingService(newMockSettingRepo())
	for _, input := range []struct{ text, target string }{{"你好", "ja"}, {"你好", "zh-CN"}, {" ", "en-US"}, {strings.Repeat("文", 12001), "en-US"}, {"你好", "en-US"}} {
		if _, err := s.Translate(context.Background(), input.text, input.target); err == nil {
			t.Fatal(input.target)
		}
	}
}
