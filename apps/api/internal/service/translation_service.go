package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dujiao-next/internal/models"
)

const TranslationSettingKey = "translation_config"

// TranslationSetting 的密钥仅供服务端调用，HTTP 响应必须使用 Public。
type TranslationSetting struct {
	Enabled    bool   `json:"enabled"`
	AutoOnSave bool   `json:"auto_on_save"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	APIKey     string `json:"api_key"`
}

func (s TranslationSetting) Public() map[string]interface{} {
	return map[string]interface{}{"enabled": s.Enabled, "auto_on_save": s.AutoOnSave, "base_url": s.BaseURL, "model": s.Model, "has_api_key": s.APIKey != ""}
}

func (s *SettingService) GetTranslationSetting() (TranslationSetting, error) {
	result := TranslationSetting{AutoOnSave: true, BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-mt-flash"}
	raw, err := s.GetByKey(TranslationSettingKey)
	if err != nil {
		return result, errors.New("读取翻译配置失败")
	}
	if raw != nil {
		data, _ := json.Marshal(raw)
		if json.Unmarshal(data, &result) != nil {
			return result, errors.New("翻译配置格式错误")
		}
	}
	return result, nil
}

func (s *SettingService) SaveTranslationSetting(next TranslationSetting, clearKey bool) (TranslationSetting, error) {
	current, err := s.GetTranslationSetting()
	if err != nil {
		return next, err
	}
	next.BaseURL = strings.TrimRight(strings.TrimSpace(next.BaseURL), "/")
	next.Model = strings.TrimSpace(next.Model)
	next.APIKey = strings.TrimSpace(next.APIKey)
	if clearKey {
		next.APIKey = ""
	} else if next.APIKey == "" {
		next.APIKey = current.APIKey
	}
	if err = validateTranslationSetting(next); err != nil {
		return next, err
	}
	data, _ := json.Marshal(next)
	var value models.JSON
	_ = json.Unmarshal(data, &value)
	if _, err = s.Update(TranslationSettingKey, value); err != nil {
		return next, errors.New("保存翻译配置失败")
	}
	return next, nil
}

func validateTranslationSetting(s TranslationSetting) error {
	u, err := url.Parse(s.BaseURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || strings.ContainsAny(s.BaseURL, "\r\n") {
		return errors.New("翻译接口地址必须为不含凭据、查询参数的 HTTPS Base URL")
	}
	if strings.HasSuffix(u.Path, "/chat/completions") {
		return errors.New("请填写 Base URL，不要包含 /chat/completions")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !translationPublicIP(ip) {
		return errors.New("翻译接口不允许使用内网地址")
	}
	if s.Model == "" || len(s.Model) > 128 {
		return errors.New("请填写有效的翻译模型名称")
	}
	if len(s.APIKey) > 4096 || strings.ContainsAny(s.APIKey, "\r\n") {
		return errors.New("API Key 格式错误")
	}
	if s.Enabled && s.APIKey == "" {
		return errors.New("启用翻译前请填写 API Key")
	}
	return nil
}

func translationPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !(ip.To4() != nil && ip.To4()[0] == 100 && ip.To4()[1] >= 64 && ip.To4()[1] <= 127)
}

var translationHTTPClient = &http.Client{
	Timeout:       25 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("redirect refused") },
	Transport: &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, errors.New("no address")
			}
			for _, ip := range ips {
				if !translationPublicIP(ip.IP) {
					return nil, errors.New("private address refused")
				}
			}
			var last error
			for _, ip := range ips {
				conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			return nil, last
		},
	},
}

// 每进程限并发；不自动重试，避免不透明的重复计费。
var translationSlots = make(chan struct{}, 4)
var translationLanguages = map[string]string{"zh-CN": "Chinese", "zh-TW": "Traditional Chinese", "en-US": "English"}
var translationProtected = regexp.MustCompile("(?s)```.*?```|`[^`]*`|<[^>]+>|\\{\\{.*?\\}\\}|\\$\\{[^}]+\\}|\\{[a-zA-Z_][a-zA-Z0-9_.]*\\}|\\]\\([^)]*\\)|https?://[^\\s<>]+|%\\[[0-9]+\\][a-zA-Z]|%[sdvf]")
var translationMarker = regexp.MustCompile(`RUOMUKEEP[0-9]+TOKEN`)

func protectTranslation(text string) (string, []string, error) {
	if translationMarker.MatchString(text) {
		return "", nil, errors.New("原文包含保留的翻译占位符")
	}
	values := []string{}
	masked := translationProtected.ReplaceAllStringFunc(text, func(s string) string {
		values = append(values, s)
		return "RUOMUKEEP" + strconv.Itoa(len(values)-1) + "TOKEN"
	})
	return masked, values, nil
}

func restoreTranslation(text string, values []string) (string, error) {
	matches := translationMarker.FindAllString(text, -1)
	if len(matches) != len(values) {
		return "", errors.New("译文格式或变量发生变化，请缩短内容后重试")
	}
	// 禁止生成新的 HTML、链接或模板变量；所有结构都来自被保护的原文。
	if translationProtected.MatchString(translationMarker.ReplaceAllString(text, "")) {
		return "", errors.New("译文包含新增的格式或变量，请重试")
	}
	for i, v := range values {
		marker := "RUOMUKEEP" + strconv.Itoa(i) + "TOKEN"
		if matches[i] != marker || strings.Count(text, marker) != 1 {
			return "", errors.New("译文格式或变量顺序发生变化，请重试")
		}
		text = strings.Replace(text, marker, v, 1)
	}
	return text, nil
}

func (s *SettingService) Translate(ctx context.Context, text, target string, useCache ...bool) (string, error) {
	if target == "zh-CN" || translationLanguages[target] == "" {
		return "", errors.New("目标语言仅支持繁体中文和英文")
	}
	if strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 12000 {
		return "", errors.New("每个翻译字段需为 1–12000 字符，请拆分过长内容")
	}
	setting, err := s.GetTranslationSetting()
	if err != nil {
		return "", err
	}
	if !setting.Enabled || setting.APIKey == "" {
		return "", errors.New("请先在系统设置中配置并启用 AI 翻译")
	}
	if err = validateTranslationSetting(setting); err != nil {
		return "", err
	}
	key := translationCacheKey(setting, text, target)
	if len(useCache) > 0 && useCache[0] {
		if !setting.AutoOnSave {
			return "", errors.New("自动翻译已关闭，请重新保存")
		}
		cached, cacheErr := s.cachedTranslation(key)
		if cacheErr != nil {
			return "", cacheErr
		}
		if cached != "" {
			return cached, nil
		}
	}
	select {
	case translationSlots <- struct{}{}:
		defer func() { <-translationSlots }()
	default:
		return "", errors.New("翻译任务繁忙，请稍后重试")
	}
	translated, err := requestTranslation(ctx, translationHTTPClient, setting, text, target)
	if err != nil {
		return "", err
	}
	if err = s.rememberTranslation(key, translated); err != nil {
		return "", err
	}
	return translated, nil
}

func requestTranslation(ctx context.Context, client *http.Client, setting TranslationSetting, text, target string) (string, error) {
	masked, protected, err := protectTranslation(text)
	if err != nil {
		return "", err
	}
	payload := map[string]interface{}{
		"model": setting.Model, "stream": false,
		"messages":            []map[string]string{{"role": "user", "content": masked}},
		"translation_options": map[string]string{"source_lang": "Chinese", "target_lang": translationLanguages[target], "domains": "E-commerce website content. Preserve every RUOMUKEEP<number>TOKEN placeholder exactly, in its original order. Preserve Markdown formatting. Translate only the content, without explanations."},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("翻译接口地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+setting.APIKey)
	res, err := client.Do(req)
	if err != nil {
		return "", errors.New("翻译请求失败或超时，请检查接口连接")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("翻译服务返回 HTTP %d，请检查密钥、模型权限或额度", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 256*1024+1))
	if err != nil || len(data) > 256*1024 {
		return "", errors.New("翻译响应过大或读取失败")
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Choices) != 1 {
		return "", errors.New("翻译服务响应格式错误")
	}
	choice := result.Choices[0]
	if choice.FinishReason != "stop" || strings.TrimSpace(choice.Message.Content) == "" {
		return "", errors.New("译文为空或未完整生成，请缩短内容后重试")
	}
	return restoreTranslation(choice.Message.Content, protected)
}
