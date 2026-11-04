# Agent Note: 共享导购首页与商品目录边界

Status: implemented

## Problem

用户要求首页参考 getgpt.pro 的大标题、明确选购入口、服务说明、流程和问答布局。现有 classic/vault 首页各自维护横幅与商品展示，list 模式还在 /products 和 /categories/:slug 复用 Home，直接替换会影响分类筛选。首页不能复制参考站的客户数、评价、价格或到账/退款承诺。

## Decision

仅根路由使用共享 StorefrontHome；保留两套外壳及原目录组件。将原 Home 的清除分类目标改为 products，防止回到导购首页。首页复用公开商品/分类/文章 API、现有价格库存规则、快捷购买、营业时间、公告及横幅轮播。简体、繁体、英文文案一致；分类跳转完整目录，不对首页商品切片伪装全量筛选；加载失败提供重试，空库存仍可查看商品但不能快捷购买。品牌来自当前租户配置。

## Existing capabilities and note audit

已核对 FEATURE_MAP、router/index.ts、模板 registry、两套 Home、useProductList、useBannerCarousel、ProductCard、useProductLabels 及公开 API。搜索首页/模板/home/storefront/品牌未发现已有归属笔记；container-release 仅涉及发布，qwen-content-translation 仅涉及后台内容翻译，均无关，保留原状。

## Alternatives considered

- 分别重做两个 Home：可以完全贴合各模板，但数据/FAQ/异常处理会重复，且影响列表路由。
- 仅改 classic：改动较小，但 vault 站点不会得到新首页。
- 独立静态营销站：排版自由，但价格、库存、租户品牌与购买入口容易脱节，需要另一个部署。本项目使用已有 Vue 前台中的共享页面。

## Validation

用户前台 vue-tsc 与 Vite 生产构建通过；现有 33 项 Node 前端测试通过。Playwright 使用本地模拟公开 API（不下真实订单），验证 classic/vault × 简繁英 × 320/390/768/1440 共 24 组无横向溢出；缺货禁购、促销价格、快捷购买弹窗、FAQ 键盘操作、游客订单导航、租户品牌和币种、文章隐藏开关、两模板列表分类跳转/清除/搜索空态、商品失败重试/空态、公告展示与关闭、横幅切换及链接均通过。暗色 CSS 作用域经浏览器核对修正。笔记门禁与 git diff --check 通过。

## Consequences

首页与目录分开后，收藏的根路由显示导购首页；完整商品仍在 /products。新页复用现有价格/库存规则、公开 API 与购买弹窗，没有新后端或新部署服务；新增卡片只负责展示与调用既有购买入口。固定版本继续按 card-<提交 SHA> 发布，发布不自动部署生产。浏览器验收使用合成商品，不能代替真实支付、交付或上线验收。
