<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { useAdminAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { notifyError, notifySuccess } from '@/utils/notify'
import { translateForm, type TranslationTarget } from '@/utils/translation'
const props = defineProps<{ value: object; disabled?: boolean; automaticStatus?: string }>()
const emit = defineEmits<{ busy: [value: boolean] }>()
const { t } = useI18n()
const auth = useAdminAuthStore()
const busy = ref(false)
const overwrite = ref(false)
const targets = ref<TranslationTarget[]>(['zh-TW', 'en-US'])
const done = ref(0)
const total = ref(0)
let active = true
onBeforeUnmount(() => { active = false; emit('busy', false) })
async function run() {
 if (busy.value || props.disabled || !targets.value.length) return
 busy.value = true; emit('busy', true)
 try {
  const count = await translateForm(props.value, targets.value, overwrite.value, async (text, target) => {
   const res = await api.post('/admin/translations', { text, target_locale: target })
   return res.data.data.text as string
  }, (n, all) => { done.value = n; total.value = all }, () => active, () => props.value)
  if (active) notifySuccess(t(count ? 'translation.success' : 'translation.empty', { count }))
 } catch (error: any) {
  if (active && !error?.__notified) notifyError(t(['translationChanged','translationEmpty'].includes(error?.message) ? `translation.${error.message}` : 'translation.failed'))
 } finally { busy.value = false; emit('busy', false) }
}
</script>
<template>
 <div v-if="auth.hasPermission('POST:/admin/translations')" class="rounded-lg border border-border bg-muted/20 p-3 space-y-2">
  <p v-if="automaticStatus" role="status" class="text-sm font-medium">{{ automaticStatus }}</p>
  <p class="text-xs text-muted-foreground">{{ t('translation.description') }}</p>
  <details>
   <summary class="cursor-pointer text-xs text-muted-foreground">{{ t('translation.manualTools') }}</summary>
  <div class="mt-2 flex flex-wrap items-center gap-3 text-sm">
   <Button type="button" variant="outline" size="sm" :disabled="busy || disabled || !targets.length" @click="run">{{ busy ? t('translation.progress', { done, total }) : t('translation.translate') }}</Button>
   <label class="flex items-center gap-1"><input v-model="targets" type="checkbox" value="zh-TW" :disabled="busy || disabled">{{ t('admin.common.lang.zhTW') }}</label>
   <label class="flex items-center gap-1"><input v-model="targets" type="checkbox" value="en-US" :disabled="busy || disabled">{{ t('admin.common.lang.enUS') }}</label>
   <label class="flex items-center gap-1"><input v-model="overwrite" type="checkbox" :disabled="busy || disabled">{{ t('translation.overwrite') }}</label>
  </div>
  </details>
 </div>
</template>
