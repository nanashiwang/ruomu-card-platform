export type TranslationTarget = 'zh-TW' | 'en-US'
export type TranslationTask = { text: string; target: TranslationTarget; apply: (text: string) => void }
const record = (value: unknown): value is Record<string, any> => value !== null && typeof value === 'object'
export const emptyTranslation = (value: unknown) => typeof value !== 'string' || !value.replace(/<[^>]*>/g, '').replace(/&nbsp;/gi, '').trim() && !/<(img|video|table|hr)\b/i.test(value)

// 只访问以 zh-CN 为源的多语言节点，普通 URL、脚本、标识和配置不送往模型。
export function collectTranslationTasks(root: unknown, targets: TranslationTarget[], overwrite: boolean, includeEmpty = false): TranslationTask[] {
  const tasks: TranslationTask[] = []
  const visit = (value: unknown) => {
    if (!record(value)) return
    if ('zh-CN' in value) {
      const localized = value
      const walk = (source: unknown, path: string[]) => {
        if (typeof source === 'string') {
          if (emptyTranslation(source) && !includeEmpty) return
          for (const target of targets) {
            let current: any = localized[target]
            for (const key of path) current = current?.[key]
            if (!overwrite && !emptyTranslation(current)) continue
            tasks.push({ text: source, target, apply: (text) => {
              if (!path.length) { localized[target] = text; return }
              if (!record(localized[target])) localized[target] = {}
              let parent = localized[target]
              for (const key of path.slice(0, -1)) {
                if (!record(parent[key])) parent[key] = {}
                parent = parent[key]
              }
              parent[path[path.length - 1]!] = text
            } })
          }
        } else if (record(source)) {
          for (const [key, child] of Object.entries(source)) walk(child, [...path, key])
        }
      }
      walk(localized['zh-CN'], [])
      return
    }
    for (const child of Object.values(value)) visit(child)
  }
  visit(root)
  return tasks
}

export async function translateForm(root: unknown, targets: TranslationTarget[], overwrite: boolean,
  translate: (text: string, target: TranslationTarget) => Promise<string>,
  progress: (done: number, total: number) => void, active: () => boolean = () => true, currentRoot: () => unknown = () => root, includeEmpty = false): Promise<number> {
  const snapshot = JSON.stringify(root)
  const tasks = collectTranslationTasks(root, targets, overwrite, includeEmpty)
  const results: string[] = []
  const cache = new Map<string, string>()
  progress(0, tasks.length)
  for (const task of tasks) {
    if (!active()) return 0
    const key = JSON.stringify([task.text, task.target])
    let text = cache.get(key)
    if (includeEmpty && emptyTranslation(task.text)) text = ''
    if (text === undefined) { text = await translate(task.text, task.target); cache.set(key, text) }
    if (!text.trim() && !(includeEmpty && emptyTranslation(task.text))) throw new Error('translationEmpty')
    results.push(text)
    progress(results.length, tasks.length)
  }
  if (!active()) return 0
  if (JSON.stringify(currentRoot()) !== snapshot) throw new Error('translationChanged')
  tasks.forEach((task, index) => task.apply(results[index]!))
  return tasks.length
}


// 保存前配置检查也在竞态保护范围内，取消或失败都不能继续业务保存。
export async function prepareAutomaticTranslation(
  root: unknown,
  settings: () => Promise<{ enabled: boolean; auto_on_save: boolean }>,
  translate: (text: string, target: TranslationTarget) => Promise<string>,
  progress: (done: number, total: number) => void,
  active: () => boolean = () => true,
  currentRoot: () => unknown = () => root,
): Promise<void> {
  const snapshot = JSON.stringify(root)
  const config = await settings()
  if (!active()) throw new Error('translationCanceled')
  if (JSON.stringify(currentRoot()) !== snapshot) throw new Error('translationChanged')
  if (!config.enabled || !config.auto_on_save) return
  await translateForm(root, ['zh-TW', 'en-US'], true, translate, progress, active, currentRoot, true)
  if (!active()) throw new Error('translationCanceled')
}
