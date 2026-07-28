import test from 'node:test'
import assert from 'node:assert/strict'
import { isGptSubscriptionProduct } from '../src/utils/gptSubscription.ts'

test('identifies GPT subscription products from title, slug, or category context', () => {
  assert.equal(isGptSubscriptionProduct('ChatGPT Plus 代充'), true)
  assert.equal(isGptSubscriptionProduct('正价订阅', 'gpt-pro-recharge'), true)
  assert.equal(isGptSubscriptionProduct('Plus 正价消费', '', 'GPT 订阅'), true)
})

test('does not show subscription warnings on unrelated GPT or non-GPT products', () => {
  assert.equal(isGptSubscriptionProduct('GPT API Key 按量计费'), false)
  assert.equal(isGptSubscriptionProduct('Gemini Pro 订阅'), false)
  assert.equal(isGptSubscriptionProduct('Microsoft 365 会员'), false)
  assert.equal(isGptSubscriptionProduct(undefined, null, ''), false)
})
