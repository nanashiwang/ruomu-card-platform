# Agent Note: 主域名下的管理后台路径

Status: implemented

## Problem

生产同时存在用户主域名与历史管理入口 `rm.meta-api.vip:8443`。用户要求以后以 `rcard.cc` 为主，并通过 `https://rcard.cc/admin/` 访问管理后台。原管理员镜像以根路径构建，直接把 Nginx `/admin` 代理到该镜像会导致 `/assets/*` 与用户前台静态资源冲突。

## Decision

管理后台仍保持独立构建、独立容器和独立 API 权限边界，但构建时显式使用 `/admin/` 作为 Vite base；主域名网关将 `/admin/`（含静态资源和 SPA 深层路由）代理到 admin 容器并剥离前缀，用户根路径继续代理 user 容器。旧 `rm.meta-api.vip:8443` 入口不再作为生产入口，保留 `admin.rcard.cc` 仅作为可选兼容入口时必须明确重定向到主域名路径。

## Existing capabilities and note audit

已核对 FEATURE_MAP、admin/user router、admin Vite base、独立 Docker 镜像、API fullstack `/admin` 方案和现有生产 Nginx。fullstack 方案能提供路径挂载，但会改变 API 镜像和生产运行形态；本次复用现有独立 admin 容器，仅扩展其静态资源 base 与网关路径，避免把前端合并进 Go API。现有 container-release 笔记负责不可变镜像/数据库门禁，与本路径契约互补。

## Alternatives considered

- 继续使用 `rm.meta-api.vip:8443`：无需迁移，但不符合主域名要求，且历史端口入口容易与用户域名混淆。
- 直接代理未改 base 的 admin 镜像：实现表面简单，但 `/assets` 会命中 user 容器，登录页会出现资源错配。
- 迁移到 API fullstack 二进制：路径支持完整，但扩大 API 构建、镜像和回滚范围，超出本次域名迁移必要范围。

## Validation

验证 admin 前端使用 `/admin/` 输出资源 URL；构建、类型检查和既有翻译测试通过。发布前需通过三端镜像、部署路由和 Nginx 配置检查，并在线验证 `https://rcard.cc/admin/login` 与 API 登录请求；旧 8443 端口关闭后不得再报告为可用入口。

## Consequences

主域名可同时承载用户 `/` 和后台 `/admin/`，静态资源不与 user 冲突；管理员深层路由刷新由 admin 容器 SPA fallback 处理。admin 镜像不能再假设根路径部署，后续如恢复独立子域名需使用 `/admin/` base 或重新构建。旧入口关闭会使旧书签失效，这是用户明确要求的迁移结果。
