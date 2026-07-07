# 若木云卡 Ruomu Card

若木云卡是一个基于 Dujiao-Next 二开的虚拟卡、卡密、数字商品与人工交付平台。

本项目采用 monorepo 组织方式：

- apps/api：后端 API 服务，基于 Dujiao-Next 服务端；
- apps/user：用户前台，负责商品展示、下单、支付和订单查询；
- apps/admin：管理后台，负责商品、订单、用户、支付、权限和后续供应商后台能力；
- docs：二开文档、数据库变更记录、接口规范和业务设计；
- deploy：部署配置；
- scripts：开发、构建和部署脚本。

## 快速开始

### 环境要求

- Go 1.26.3 或支持自动 toolchain 下载的新版 Go；
- Node.js 20+；
- npx；
- Docker / Docker Compose 可选，用于容器化开发或部署；
- 前端固定使用 `pnpm@10.34.3`，根目录脚本会通过 `npx pnpm@10.34.3` 调用，避免全局 pnpm 版本不一致。

### 本地热更新开发

```bash
./scripts/dev.sh
```

脚本会自动完成：

- 如果 `/apps/api/config.yml` 不存在，从 `/apps/api/config.ruomu.dev.yml.example` 复制一份；
- 安装 user/admin 前端依赖；
- 启动 API、用户前台、管理后台。

本地访问地址：

- API 健康检查：http://127.0.0.1:8080/health
- 用户前台：http://127.0.0.1:5173
- 管理后台：http://127.0.0.1:5174
- 本地默认管理员：admin / Admin123456

说明：本地热更新模式默认关闭 Redis 与队列，只启动 API 模式，适合页面开发和基础接口联调。涉及异步任务、发货队列、库存同步时，请使用 Docker 开发环境或手动启用 Redis/Queue。

### Docker 开发环境

```bash
docker compose -f deploy/docker-compose.dev.yml up -d --build
```

默认端口：

- API：http://127.0.0.1:8080
- 用户前台：http://127.0.0.1:5173
- 管理后台：http://127.0.0.1:5174
- Redis：127.0.0.1:6379

Docker 开发环境会启用 Redis 与 worker，更接近完整运行形态。

### 构建验证

```bash
./scripts/build.sh
```

默认会执行：

- `apps/api`：`go test ./...` + `go build`；
- `apps/user`：安装依赖并构建；
- `apps/admin`：安装依赖并构建。

如只想构建不跑后端测试：

```bash
RUN_TESTS=0 ./scripts/build.sh
```

### 生产部署草案

```bash
cp .env.example .env
# 修改 .env 中的 APP_SECRET_KEY、JWT_SECRET、USER_JWT_SECRET、DEFAULT_ADMIN_PASSWORD 等生产配置
./scripts/deploy.sh
```

当前生产 compose 是基础草案，默认暴露：

- `USER_PORT`：用户前台；
- `ADMIN_PORT`：管理后台；
- `API_PORT`：API 服务。

正式上线前建议接入外层 HTTPS 网关，并按域名拆分用户前台和管理后台。

## 阶段目标

### 第一阶段：跑通原系统

- 拉取 Dujiao-Next 三端代码；
- 本地跑通 API、用户前台、管理后台；
- 熟悉商品、订单、支付、卡密交付流程。

### 第二阶段：供应商代发模块

- 增加供应商账号；
- 商品绑定供应商；
- 订单自动分配给供应商；
- 供应商只看到自己的订单；
- 供应商提交卡密或交付内容；
- 客户端不展示供应商信息。

### 第三阶段：平台运营能力

- 供应商结算；
- 售后工单；
- 风控审核；
- 订单异常处理；
- 财务统计；
- 平台运营看板。

## 核心原则

代码仓库合并，应用边界保留。

不要将 api、user、admin 混成一个应用。三端保持独立开发、独立构建、独立部署，由根目录统一管理文档、部署和 AI 开发规范。
