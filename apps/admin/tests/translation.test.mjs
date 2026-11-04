import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import ts from 'typescript'
const source = await readFile(new URL('../src/utils/translation.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } }).outputText
const { collectTranslationTasks, translateForm } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`)
const progress = () => {}

test('fills missing languages while preserving manual values and nonlocalized config', async () => {
 const form = { title: { 'zh-CN': '商品', 'en-US': 'Manual' }, url: 'https://example.com', price: 12, items: [{ 'zh-CN': '说明' }] }
 const calls = []
 await translateForm(form, ['zh-TW', 'en-US'], false, async (text, target) => { calls.push([text, target]); return `${target}:${text}` }, progress)
 assert.equal(form.title['en-US'], 'Manual'); assert.equal(form.title['zh-CN'], '商品')
 assert.equal(form.items[0]['en-US'], 'en-US:说明'); assert.equal(calls.length, 3)
 assert.equal(form.url, 'https://example.com'); assert.equal(form.price, 12)
})
test('nested language objects, explicit overwrite and empty rich text', async () => {
 const form = { templates: { 'zh-CN': { subject: '标题', body: '{{order_no}}' }, 'en-US': { subject: 'Old' } }, blank: { 'zh-CN': '<p><br></p>' } }
 await translateForm(form, ['en-US'], true, async text => `Translated ${text}`, progress)
 assert.deepEqual(form.templates['en-US'], { subject: 'Translated 标题', body: 'Translated {{order_no}}' })
 assert.equal(collectTranslationTasks(form.blank, ['en-US'], false).length, 0)
})
test('a later failure never applies partial translations', async () => {
 const form = { a: { 'zh-CN': '一' }, b: { 'zh-CN': '二' } }; const before = JSON.stringify(form)
 await assert.rejects(translateForm(form, ['en-US'], false, async text => { if (text === '二') throw Error('offline'); return 'One' }, progress), /offline/)
 assert.equal(JSON.stringify(form), before)
})
test('edits or replacing the form while waiting are preserved', async () => {
 const form = { title: { 'zh-CN': '一' } }
 await assert.rejects(translateForm(form, ['en-US'], false, async () => { form.title['zh-CN'] = '新内容'; return 'One' }, progress), /translationChanged/)
 assert.equal(form.title['en-US'], undefined)
 await assert.rejects(translateForm(form, ['en-US'], false, async () => 'One', progress, () => true, () => ({ title: {} })), /translationChanged/)
})
test('closed editor does not apply or continue requests; duplicate text is reused', async () => {
 const form = { a: { 'zh-CN': '一' }, b: { 'zh-CN': '一' } }; let calls = 0
 await translateForm(form, ['en-US'], false, async () => { calls++; return 'One' }, progress)
 assert.equal(calls, 1); assert.equal(form.b['en-US'], 'One')
 let active = true; const closed = { a: { 'zh-CN': '一' }, b: { 'zh-CN': '二' } }
 calls = 0
 await translateForm(closed, ['en-US'], false, async () => { calls++; active = false; return 'One' }, progress, () => active)
 assert.equal(calls, 1); assert.equal(closed.a['en-US'], undefined)
})
