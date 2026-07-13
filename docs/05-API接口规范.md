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
