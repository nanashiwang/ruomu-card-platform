# Agent Note: Qwen 管理端多语言内容翻译

Status: implemented

## Problem

商品、分类、文章、轮播、会员等级及站点配置已有 zh-CN/zh-TW/en-US 内容映射和独立保存接口，运营仍需逐语言填写。设置通用接口允许按 key 读写，因此新模型密钥不能直接沿用该接口暴露。

## Decision

复用 SettingService 和既有内容表单，增加独立 translation_config 设置及 Qwen-MT 服务端代理。提供单条中文到目标语言的管理端接口、模型配置与连通测试。编辑器共享按钮递归发现多语言字段（含语言下对象形式），仅补空值或显式覆盖，整轮成功且原表单未变才回填；用户检查后自行保存。不在用户页面访问时调用模型，不后台修改既有内容。

密钥读响应只提供 has_api_key；空密钥保留、显式清除。通用 settings 对新 key 拒绝访问，独立配置权限与翻译权限分离。请求使用 Qwen-MT 单 user 消息及 translation_options；保护 HTML、链接、代码和模板变量，拒绝丢失占位符和截断输出。限制文本、时间、并发和响应体，禁止重定向及私网端点。

## Existing capabilities and note audit

已检查 FEATURE_MAP、SettingService、admin_setting、RBAC 预设、前后台语言映射与各内容编辑页。检索活跃笔记翻译、translation、locale、setting、设置无命中；已有发布和工作流笔记不持有本决定。保持 Go API、admin、user 三端边界，无数据库结构迁移。

## Alternatives considered

- 保存时自动翻译能少一次点击，但失败会阻塞业务保存，覆盖人工内容难以判断；采用显式生成、现有保存。
- 单次将整个表单 JSON 发给通用聊天模型较快，但 Qwen-MT 为单段翻译设计，JSON 字段和结构容易漂移；逐字段调用并只传待译文本。
- 只在商品页新增按钮工作量小，但站点介绍、公告和导航仍重复人工录入；复用共享组件接入已有多语言入口。

## Verification

`./scripts/build.sh` 通过全量 Go 测试、API 构建及 user/admin 类型检查与生产构建；部署工具 40 项测试通过。模拟上游测试覆盖 Qwen 请求协议、繁中/英文语种、错误与取消、截断、空响应、响应限额、格式保护、密钥保留/清除和通用设置旁路。RBAC 测试验证 operations/integration 仅翻译、system_admin 可配置、只读/客服/财务不能调用。

管理端构建运行 5 项 Node 原生回归测试；复用已有 TypeScript 编译器，不引入新运行时依赖。Playwright 在真实管理前端和模拟 API 上验证分类补空/显式覆盖、翻译中禁用保存、编辑竞态、失败不回填、配置页面密钥不回显和测试入口；390px 移动布局已检查。

发布沿用 card-完整 SHA 的现有 GitHub Actions，不修改版本策略或自动部署。

## Consequences

复用既有存储和编辑流程，不迁移数据库结构，也不在保存时引入外部依赖。代价是多一步生成/检查操作与逐字段请求耗时；大批量离线翻译应另行采用任务队列。未提供真实密钥，模拟接口通过不等于真实模型验收。长文本逐字段调用耗时且产生供应商计费；模型上下文限制导致失败时保持原文。只生成已有站点语言，新增站点语种仍需前后台语言支持。
