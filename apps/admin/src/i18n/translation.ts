export const translationMessages = {
 'zh-CN': {
  title:'AI 翻译', description:'填写简体中文后，一键生成其他语言；检查译文后再保存。内容会发送到配置的翻译服务，按供应商规则计费。',
  translate:'从中文翻译', overwrite:'覆盖已有译文', progress:'翻译中 {done}/{total}', success:'已生成 {count} 项译文，请检查后保存', empty:'没有需要翻译的内容，请填写中文或选择覆盖已有译文',
  translationChanged:'翻译期间表单已修改，结果未应用，请重新翻译', translationEmpty:'翻译返回空内容，未修改表单', failed:'翻译失败，表单保持不变',
  enabled:'启用 AI 翻译', baseURL:'接口 Base URL', model:'翻译模型', apiKey:'API Key', keySaved:'已保存密钥，留空保持不变', keyEmpty:'尚未配置密钥', clearKey:'删除已保存密钥（需关闭翻译）',
  configHint:'使用 Qwen-MT 兼容接口，Base URL 以 /v1 结尾。密钥与接口地域需匹配；支持保留 translation_options 参数的中转服务。',
  save:'保存翻译配置', saved:'翻译配置已保存', test:'测试已保存配置', testSource:'欢迎来到若木云卡', testResult:'测试译文', saveFirst:'测试使用已保存配置，请先保存更改。',
 },
 'zh-TW': {
  title:'AI 翻譯', description:'填寫簡體中文後，一鍵產生其他語言；檢查譯文後再儲存。內容會傳送至設定的翻譯服務，依供應商規則計費。',
  translate:'從中文翻譯', overwrite:'覆蓋已有譯文', progress:'翻譯中 {done}/{total}', success:'已產生 {count} 項譯文，請檢查後儲存', empty:'沒有需要翻譯的內容，請填寫中文或選擇覆蓋已有譯文',
  translationChanged:'翻譯期間表單已修改，結果未套用，請重新翻譯', translationEmpty:'翻譯回傳空內容，未修改表單', failed:'翻譯失敗，表單保持不變',
  enabled:'啟用 AI 翻譯', baseURL:'介面 Base URL', model:'翻譯模型', apiKey:'API Key', keySaved:'已儲存金鑰，留空保持不變', keyEmpty:'尚未設定金鑰', clearKey:'刪除已儲存金鑰（需關閉翻譯）',
  configHint:'使用 Qwen-MT 相容介面，Base URL 以 /v1 結尾。金鑰與介面地域需相符；支援保留 translation_options 參數的轉送服務。',
  save:'儲存翻譯設定', saved:'翻譯設定已儲存', test:'測試已儲存設定', testSource:'欢迎来到若木云卡', testResult:'測試譯文', saveFirst:'測試使用已儲存設定，請先儲存變更。',
 },
 'en-US': {
  title:'AI translation', description:'Write Simplified Chinese, generate other languages, then review and save. Content is sent to the configured provider and may incur charges.',
  translate:'Translate from Chinese', overwrite:'Replace existing translations', progress:'Translating {done}/{total}', success:'Generated {count} translations. Review and save.', empty:'Nothing to translate. Enter Chinese content or allow replacement.',
  translationChanged:'The form changed during translation. Results were not applied. Please try again.', translationEmpty:'Translation returned empty content. No changes applied.', failed:'Translation failed. The form is unchanged.',
  enabled:'Enable AI translation', baseURL:'API Base URL', model:'Translation model', apiKey:'API Key', keySaved:'Key saved; leave blank to keep it', keyEmpty:'No key configured', clearKey:'Delete saved key (disable translation first)',
  configHint:'Use a Qwen-MT compatible endpoint ending in /v1. Match the key and endpoint region. Proxies must forward translation_options.',
  save:'Save translation settings', saved:'Translation settings saved', test:'Test saved settings', testSource:'欢迎来到若木云卡', testResult:'Test translation', saveFirst:'Tests use saved settings. Save changes first.',
 },
}
