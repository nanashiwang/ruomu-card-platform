<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { userApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { invoiceIneligibleReason, type InvoiceOrder } from '@/utils/invoice'
const props = defineProps<{ order: InvoiceOrder & { order_no: string } }>()
const { t } = useI18n()
const item = ref<any>(null)
const loaded = ref(false), loading = ref(false), busy = ref(false), error = ref(''), success = ref('')
const form = reactive({ kind: 'personal', title: '', tax_id: '', email: '', note: '' })
const path = computed(() => `/orders/${encodeURIComponent(props.order.order_no)}/invoice`)
const ineligibleReason = computed(() => invoiceIneligibleReason(props.order))
const eligible = computed(() => !ineligibleReason.value)
const canApply = computed(() => loaded.value && eligible.value && (!item.value || item.value.status === 'rejected'))
async function load() {
  loading.value = true; error.value = ''
  try {
    item.value = (await userApi.get(path.value)).data.data
    if (item.value?.status === 'rejected') {
      for (const key of Object.keys(form) as (keyof typeof form)[]) form[key] = item.value[key] || ''
    }
    loaded.value = true
  } catch (e: any) { error.value = e.message || t('invoice.error'); loaded.value = false }
  finally { loading.value = false }
}
async function submit() {
  if (busy.value || !canApply.value) return
  busy.value = true; error.value = ''; success.value = ''
  try {
    item.value = (await userApi.post(path.value, form)).data.data
    success.value = t('invoice.submitted')
  } catch (e: any) { error.value = e.message || t('invoice.error') }
  finally { busy.value = false }
}
async function download() {
  busy.value = true; error.value = ''
  try {
    const response = await userApi.get(`${path.value}/download`, { blob: true })
    const url = URL.createObjectURL(response.data)
    const a = document.createElement('a'); a.href = url; a.download = item.value.file_name; a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (e: any) { error.value = e.message || t('invoice.error') }
  finally { busy.value = false }
}
onMounted(load)
</script>

<template>
  <section class="rounded-2xl border bg-card p-6 space-y-4" :aria-busy="busy || loading">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-lg font-bold">{{ t('invoice.title') }}</h2>
      <Button type="button" variant="outline" size="sm" :disabled="busy || loading" @click="load">{{ t('invoice.refresh') }}</Button>
    </div>
    <p v-if="eligible" class="text-sm text-muted-foreground">{{ t('invoice.hint') }}</p>
    <p v-if="ineligibleReason" class="text-sm text-muted-foreground">{{ t(ineligibleReason) }}</p>
    <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    <p v-if="success" role="status" class="text-sm text-green-600">{{ success }}</p>
    <p v-if="loading" class="text-sm">{{ t('invoice.busy') }}</p>
    <template v-else-if="loaded">
      <div v-if="item" class="text-sm space-y-2 break-words">
        <p>{{ t('invoice.status') }}：<strong>{{ t(`invoice.${item.status}`) }}</strong></p>
        <p>{{ t('invoice.heading') }}：{{ item.title }}</p>
        <p>{{ t('invoice.amount') }}：{{ item.amount }} {{ item.currency }}</p>
        <p>{{ t('invoice.email') }}：{{ item.email }}</p>
        <p v-if="item.rejection_reason" class="text-destructive">{{ t('invoice.reason') }}：{{ item.rejection_reason }}</p>
        <template v-if="item.status === 'issued'">
          <p>{{ t('invoice.mail') }}：{{ t(`invoice.mail_${item.mail_status}`) }}</p>
          <Button type="button" :disabled="busy" variant="outline" size="sm" @click="download">{{ t('invoice.download') }}</Button>
        </template>
      </div>
      <p v-if="!item && !eligible" class="text-sm text-muted-foreground">{{ t('invoice.empty') }}</p>
      <form v-if="canApply" class="grid gap-4 sm:grid-cols-2" @submit.prevent="submit">
        <label class="text-sm space-y-2">{{ t('invoice.kind') }}
          <select v-model="form.kind" :disabled="busy" class="block w-full rounded-md border bg-background px-3 py-2"><option value="personal">{{ t('invoice.personal') }}</option><option value="company">{{ t('invoice.company') }}</option></select>
        </label>
        <label class="text-sm space-y-2">{{ t('invoice.heading') }}<input v-model="form.title" :disabled="busy" required maxlength="200" class="block w-full rounded-md border bg-background px-3 py-2" /></label>
        <label v-if="form.kind === 'company'" class="text-sm space-y-2">{{ t('invoice.tax') }}<input v-model="form.tax_id" :disabled="busy" required pattern="[A-Za-z0-9]{15,20}" maxlength="20" class="block w-full rounded-md border bg-background px-3 py-2" /></label>
        <label class="text-sm space-y-2">{{ t('invoice.email') }}<input v-model="form.email" :disabled="busy" type="email" required maxlength="254" class="block w-full rounded-md border bg-background px-3 py-2" /></label>
        <label class="text-sm space-y-2 sm:col-span-2">{{ t('invoice.note') }}<textarea v-model="form.note" :disabled="busy" maxlength="1000" rows="2" class="block w-full rounded-md border bg-background px-3 py-2" /></label>
        <Button type="submit" :disabled="busy || loading" class="justify-self-start">{{ busy ? t('invoice.busy') : t(item ? 'invoice.reapply' : 'invoice.apply') }}</Button>
      </form>
    </template>
  </section>
</template>
