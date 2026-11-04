<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { useAdminAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { notifySuccess } from '@/utils/notify'
const { t } = useI18n()
const auth = useAdminAuthStore()
const submitting = ref(false)
const loaded = ref(false)
const result = ref('')
const form = reactive({ enabled:false, auto_on_save:true, base_url:'https://dashscope.aliyuncs.com/compatible-mode/v1', model:'qwen-mt-flash', api_key:'', has_api_key:false, clear_api_key:false })
async function load() {
 try { const res = await api.get('/admin/settings/translation'); Object.assign(form,res.data.data); loaded.value=true } catch { /* API client reports failures. */ }
}
async function save() {
 if (!loaded.value || submitting.value || !auth.hasPermission('PUT:/admin/settings/translation')) return
 submitting.value=true
 try { const res=await api.put('/admin/settings/translation',form); Object.assign(form,res.data.data,{api_key:'',clear_api_key:false}); notifySuccess(t('translation.saved')) } catch { /* API client reports failures. */ } finally { submitting.value=false }
}
async function test() {
 submitting.value=true; result.value=''
 try { const res=await api.post('/admin/translations',{text:t('translation.testSource'),target_locale:'en-US'}); result.value=res.data.data.text } catch { /* API client reports failures. */ } finally { submitting.value=false }
}
onMounted(load)
defineExpose({save,submitting})
</script>
<template>
 <div class="rounded-xl border border-border bg-card p-6 space-y-4">
  <h2 class="text-lg font-semibold">{{ t('translation.title') }}</h2>
  <p class="text-sm text-muted-foreground">{{ t('translation.configHint') }}</p>
  <fieldset :disabled="submitting || !loaded || !auth.hasPermission('PUT:/admin/settings/translation')" class="space-y-4 max-w-2xl">
   <div class="flex items-center gap-2"><Switch id="translation-enabled" v-model="form.enabled" /><Label for="translation-enabled">{{ t('translation.enabled') }}</Label></div>
   <div class="flex items-center gap-2"><Switch id="translation-auto" v-model="form.auto_on_save" /><Label for="translation-auto">{{ t('translation.autoOnSave') }}</Label></div>
   <p class="text-xs text-muted-foreground">{{ t('translation.autoHint') }}</p>
   <div class="space-y-2"><Label for="translation-url">{{ t('translation.baseURL') }}</Label><Input id="translation-url" v-model="form.base_url" type="url" /></div>
   <div class="space-y-2"><Label for="translation-model">{{ t('translation.model') }}</Label><Input id="translation-model" v-model="form.model" placeholder="qwen-mt-flash" /></div>
   <div class="space-y-2"><Label for="translation-key">{{ t('translation.apiKey') }}</Label><Input id="translation-key" v-model="form.api_key" type="password" autocomplete="new-password" :placeholder="t(form.has_api_key ? 'translation.keySaved' : 'translation.keyEmpty')" /></div>
   <label class="flex items-center gap-2 text-sm"><input v-model="form.clear_api_key" type="checkbox">{{ t('translation.clearKey') }}</label>
   <Button type="button" @click="save">{{ t('translation.save') }}</Button>
  </fieldset>
  <p class="text-xs text-muted-foreground">{{ t('translation.saveFirst') }}</p>
  <Button v-if="auth.hasPermission('POST:/admin/translations')" type="button" variant="outline" :disabled="submitting || !loaded" @click="test">{{ t('translation.test') }}</Button>
  <div v-if="result" class="rounded-lg bg-muted p-3"><p class="font-medium">{{ t('translation.testResult') }}</p><p class="whitespace-pre-wrap">{{ result }}</p></div>
 </div>
</template>
