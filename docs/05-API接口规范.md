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
- 请求参数：JSON：`account_email`（必填，有效邮箱）、`current_plan`（必填，套餐代码）。
- 响应字段：更新后的订单详情；订单项包含 `post_payment_info_required` 和 `post_payment_info`。
- 错误码：订单不存在、订单状态不允许、订单项无需补充资料、资料格式无效、保存失败。
- 说明：仅允许在订单项状态为 `paid` 或 `fulfilling` 时提交或修改；不接收密码、验证码、Access Token 等登录凭证。
