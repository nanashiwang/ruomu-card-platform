# API 接口规范

所有新增 API 必须记录在本文件中。

格式：

## 接口名称

- 方法：
- 路径：
- 权限：
- 请求参数：
- 响应字段：
- 错误码：
- 说明：

## 提交付款后订单资料

- 方法：`PUT`
- 路径：`/api/v1/user/orders/:order_no/items/:item_id/post-payment-info`
- 权限：已登录普通用户，仅限订单所有者。
- 请求参数：JSON：`contact_email`（必填，订单沟通邮箱）、`current_plan`（必填，套餐代码）、`order_note`（必填，最多 1000 字）。游客接口另需订单查询用的 `email` 和 `order_password`。
- 响应字段：更新后的订单详情；订单项包含 `post_payment_info_required` 和 `post_payment_info`。
- 错误码：订单不存在、订单状态不允许、订单项无需补充资料、资料格式无效、保存失败。
- 说明：仅允许在订单项状态为 `paid` 或 `fulfilling` 时提交或修改；联系邮箱仅用于订单沟通，不是 ChatGPT 账号。不接收账号、密码、验证码、Session、Access Token 等登录凭证。

## 用户专属商品价格（2026-11-10）

- `GET /api/v1/admin/users/:id/product-prices`：读取用户配置，返回数组，包含 `id/user_id/product_id/sku_id/price_amount` 和商品、规格展示信息。
- `PUT /api/v1/admin/users/:id/product-prices`：新增或覆盖同一用户、商品、规格的价格。请求 `{ "product_id": 1, "sku_id": 0, "price_amount": "35.25" }`；金额必须为正十进制字符串、最多 18 位整数和 2 位小数。`sku_id=0` 表示商品全部规格，具体规格优先。
- `DELETE /api/v1/admin/users/:id/product-prices/:price_id`：仅删除该用户的指定配置。不存在或属于其他用户均返回未找到。
- 权限：沿用管理端 JWT + RBAC；运营角色和超级管理员可写，继承只读权限的角色可读。其他角色可以通过已有权限管理单独授权。
- 错误：参数/金额/商品/规格不合法返回业务 `400`，用户或价格记录不存在返回 `404`，存储失败返回 `500`。采用现有统一响应结构。
- 规则：主站登录用户命中专属价后以此为固定单价，不叠加活动、批发和会员价，优惠券继续按原规则计算；删除恢复原定价。分销站不应用该配置。

## 登录用户商品列表与详情

- `GET /api/v1/products`、`GET /api/v1/products/:slug`：与现有 `/public/products` 列表/详情相同参数，要求有效用户 access JWT，复用商品可见性及库存逻辑。
- 主站返回当前用户适用的 `price_amount`，命中商品展示规格或 SKU 时增加 `user_price: true`，清除该 SKU 的活动/会员价格附加值；前台据此跳过对应折扣。不同规格可分别命中专属价或沿用常规定价。
- 使用 `Cache-Control: private, no-store`；不返回其他用户配置，身份仅从认证上下文取值。公开接口不返回用户专属价，游客不受影响。
- 购物车查询、订单预览和下单读取当前配置；客户端价格不参与服务端计价。历史订单不随配置更新重算。
