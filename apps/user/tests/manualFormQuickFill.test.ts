import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveManualQuickFillValue } from '../src/utils/manualFormQuickFill.ts'

test('quick fill accepts the required declaration using its exact displayed text', () => {
  const result = resolveManualQuickFillValue(
    { key: 'agreement_confirmation', type: 'text', options: [] },
    '用户协议确认',
    '我已阅读并接受本商品全部须知规则与全部协议内容',
    '',
  )
  assert.deepEqual(result, {
    handled: true,
    value: '我已阅读并接受本商品全部须知规则与全部协议内容',
  })
})

test('quick fill reuses a normalized email but does not invent ordinary required values', () => {
  assert.deepEqual(
    resolveManualQuickFillValue({ key: 'contact_email', type: 'text', options: [] }, '联系邮箱', '', ' buyer@example.com '),
    { handled: true, value: 'buyer@example.com' },
  )
  assert.deepEqual(
    resolveManualQuickFillValue({ key: 'account_name', type: 'text', options: [] }, '账号名称', '', 'buyer@example.com'),
    { handled: false },
  )
})

test('quick fill only selects checkbox options when the field is an agreement', () => {
  assert.deepEqual(
    resolveManualQuickFillValue({ key: 'terms', type: 'checkbox', options: ['同意'] }, '用户协议', '', ''),
    { handled: true, value: ['同意'] },
  )
  assert.deepEqual(
    resolveManualQuickFillValue({ key: 'preferences', type: 'checkbox', options: ['邮件', '短信'] }, '通知偏好', '', ''),
    { handled: false },
  )
})
