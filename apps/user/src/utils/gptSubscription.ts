const GPT_BRAND_PATTERN = /(?:chatgpt|(?:^|[^a-z0-9])gpt(?:[^a-z0-9]|$))/i
const SUBSCRIPTION_PATTERN = /(?:plus|pro|team|subscription|subscribe|upgrade|recharge|订阅|充值|代充|升级|会员)/i

export function isGptSubscriptionProduct(...labels: Array<unknown>): boolean {
  const searchable = labels
    .filter((label): label is string => typeof label === 'string')
    .join(' ')
    .trim()

  return GPT_BRAND_PATTERN.test(searchable) && SUBSCRIPTION_PATTERN.test(searchable)
}
