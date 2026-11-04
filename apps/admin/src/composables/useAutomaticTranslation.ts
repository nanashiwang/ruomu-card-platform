import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { useAdminAuthStore } from '@/stores/auth'
import { notifyError } from '@/utils/notify'
import { prepareAutomaticTranslation } from '@/utils/translation'

export function useAutomaticTranslation(value: () => object, editorActive: () => boolean = () => true) {
  const auth = useAdminAuthStore()
  const { t } = useI18n()
  const busy = ref(false)
  const done = ref(0)
  const total = ref(0)
  let mounted = true
  onBeforeUnmount(() => { mounted = false })
  const status = computed(() => busy.value
    ? total.value ? t('translation.progress', { done: done.value, total: total.value }) : t('translation.checking')
    : '')
  async function prepare(): Promise<boolean> {
    if (busy.value) return false
    if (!auth.hasPermission('POST:/admin/translations')) return true
    busy.value = true
    done.value = total.value = 0
    try {
      await prepareAutomaticTranslation(value(), async () => {
        const res = await api.get('/admin/settings/translation')
        return res.data.data
      }, async (text, target) => {
        const res = await api.post('/admin/translations', { text, target_locale: target, use_cache: true })
        return res.data.data.text as string
      }, (n, all) => { done.value = n; total.value = all }, () => mounted && editorActive(), value)
      return true
    } catch (error: any) {
      if (mounted && !error?.__notified) {
        const key = ['translationChanged', 'translationEmpty', 'translationCanceled'].includes(error?.message)
          ? error.message : 'autoFailed'
        notifyError(t(`translation.${key}`))
      }
      return false
    } finally {
      busy.value = false
    }
  }
  return { prepare, status }
}
