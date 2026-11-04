package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/dujiao-next/internal/models"
)

const TranslationCacheSettingKey = "translation_cache"
const translationCacheMaxEntries = 256
const translationCacheMaxBytes = 2 * 1024 * 1024

var translationCacheMu sync.Mutex

type translationCacheEntry struct {
	Text      string `json:"text"`
	UpdatedAt int64  `json:"updated_at"`
}

// 摘要包含模型和接口，切换供应商或中文变化不会复用旧结果；不存源文和密钥。
func translationCacheKey(setting TranslationSetting, text, target string) string {
	data, _ := json.Marshal([]string{setting.BaseURL, setting.Model, text, target})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *SettingService) readTranslationCache() (map[string]translationCacheEntry, error) {
	raw, err := s.GetByKey(TranslationCacheSettingKey)
	if err != nil {
		return nil, errors.New("读取翻译缓存失败，请稍后重试")
	}
	entries := map[string]translationCacheEntry{}
	if raw != nil {
		data, _ := json.Marshal(raw)
		if json.Unmarshal(data, &entries) != nil {
			return nil, errors.New("翻译缓存格式错误")
		}
	}
	return entries, nil
}

func (s *SettingService) cachedTranslation(key string) (string, error) {
	translationCacheMu.Lock()
	defer translationCacheMu.Unlock()
	entries, err := s.readTranslationCache()
	if err != nil {
		return "", err
	}
	return entries[key].Text, nil
}

func (s *SettingService) rememberTranslation(key, text string) error {
	translationCacheMu.Lock()
	defer translationCacheMu.Unlock()
	entries, err := s.readTranslationCache()
	if err != nil {
		return err
	}
	entries[key] = translationCacheEntry{Text: text, UpdatedAt: time.Now().UnixNano()}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return entries[keys[i]].UpdatedAt < entries[keys[j]].UpdatedAt })
	data, _ := json.Marshal(entries)
	for len(entries) > translationCacheMaxEntries || len(data) > translationCacheMaxBytes {
		delete(entries, keys[0])
		keys = keys[1:]
		data, _ = json.Marshal(entries)
	}
	var value models.JSON
	_ = json.Unmarshal(data, &value)
	if _, err = s.Update(TranslationCacheSettingKey, value); err != nil {
		return errors.New("保存翻译缓存失败，请稍后重试")
	}
	return nil
}
