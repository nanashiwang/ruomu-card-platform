<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { useAdminAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
const { t } = useI18n(), auth = useAdminAuthStore()
const items = ref<any[]>([]), total = ref(0), page = ref(1), status = ref(''), orderNo = ref('')
const loading = ref(false), busy = ref(false), error = ref(''), success = ref('')
const selectedId = ref<number | null>(null), reason = ref(''), file = ref<File | null>(null)
const selected = computed(() => items.value.find(x => x.id === selectedId.value))
const can = (action: string) => auth.hasPermission(`POST:/admin/invoices/:id/${action}`)
async function load(next = page.value) {
  loading.value = true; error.value = ''
  try {
    const data = (await api.get('/admin/invoices', { params: { page: next, status: status.value, order_no: orderNo.value.trim() } })).data.data
    items.value = data.items; total.value = data.total; page.value = data.page
  } catch (e: any) { error.value = e.message || t('invoice.error') }
  finally { loading.value = false }
}
function select(id: number) { selectedId.value = id; reason.value = ''; file.value = null; success.value = ''; error.value = '' }
async function act(action: string, body?: unknown) {
  if (busy.value || !selected.value) return
  busy.value = true; error.value = ''; success.value = ''
  try {
    await api.post(`/admin/invoices/${selected.value.id}/${action}`, body)
    success.value = t('invoice.success'); file.value = null
    await load()
  } catch (e: any) {
    const message = e.message || t('invoice.error')
    await load()
    error.value = message
  }
  finally { busy.value = false }
}
function upload() {
  if (!file.value) return
  if (file.value.size > 5 * 1024 * 1024) { error.value = t('invoice.pdf'); return }
  const data = new FormData(); data.append('file', file.value); void act('file', data)
}
function chooseFile(event: Event) { file.value = (event.target as HTMLInputElement).files?.[0] || null }
async function download() {
  if (!selected.value) return
  busy.value = true; error.value = ''
  try {
    const res = await api.get(`/admin/invoices/${selected.value.id}/download`, { blob: true })
    const url = URL.createObjectURL(res.data), a = document.createElement('a')
    a.href = url; a.download = selected.value.file_name; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (e: any) { error.value = e.message || t('invoice.error') }
  finally { busy.value = false }
}
onMounted(() => load())
</script>
<template>
  <div class="space-y-5" :aria-busy="loading || busy">
    <h1 class="text-2xl font-bold">{{ t('invoice.management') }}</h1>
    <form class="flex flex-wrap gap-3" @submit.prevent="load(1)">
      <input v-model="orderNo" :aria-label="t('invoice.order')" :placeholder="t('invoice.order')" :disabled="loading || busy" class="rounded-md border bg-background px-3 py-2" />
      <select v-model="status" :aria-label="t('invoice.status')" :disabled="loading || busy" class="rounded-md border bg-background px-3 py-2"><option value="">{{ t('invoice.all') }}</option><option v-for="s in ['pending','approved','rejected','issued']" :key="s" :value="s">{{ t(`invoice.${s}`) }}</option></select>
      <Button type="submit" :disabled="loading || busy">{{ t('invoice.search') }}</Button>
      <Button type="button" variant="outline" :disabled="loading || busy" @click="load()">{{ t('invoice.refresh') }}</Button>
    </form>
    <p v-if="error" role="alert" class="text-destructive">{{ error }}</p>
    <p v-if="success" role="status" class="text-green-600">{{ success }}</p>
    <div class="overflow-x-auto rounded-lg border">
      <table class="w-full text-sm text-left"><thead class="bg-muted"><tr><th class="p-3">{{ t('invoice.order') }}</th><th class="p-3">{{ t('invoice.heading') }}</th><th class="p-3">{{ t('invoice.amount') }}</th><th class="p-3">{{ t('invoice.status') }}</th><th class="p-3">{{ t('invoice.mail') }}</th><th class="p-3">{{ t('invoice.detail') }}</th></tr></thead>
        <tbody><tr v-for="row in items" :key="row.id" class="border-t"><td class="p-3"><RouterLink :to="{ path: '/orders', query: { order_id: row.order_id } }" class="text-primary underline">{{ row.order_no }}</RouterLink></td><td class="p-3">{{ row.title }}</td><td class="p-3 whitespace-nowrap">{{ row.amount }} {{ row.currency }}</td><td class="p-3">{{ t(`invoice.${row.status}`) }}</td><td class="p-3">{{ t(`invoice.mail_${row.mail_status}`) }}</td><td class="p-3"><Button size="sm" variant="outline" :disabled="busy || loading" @click="select(row.id)">{{ t('invoice.detail') }}</Button></td></tr></tbody>
      </table>
      <p v-if="!items.length && !loading && !error" class="p-8 text-center text-muted-foreground">{{ t('invoice.empty') }}</p>
      <p v-if="loading" class="p-4">{{ t('invoice.busy') }}</p>
    </div>
    <div class="flex flex-wrap items-center gap-3"><Button variant="outline" :disabled="page <= 1 || loading || busy" @click="load(page - 1)">{{ t('invoice.previous') }}</Button><span class="text-sm">{{ t('invoice.total', { total, page }) }}</span><Button variant="outline" :disabled="page * 20 >= total || loading || busy" @click="load(page + 1)">{{ t('invoice.next') }}</Button></div>
    <section v-if="selected" class="rounded-xl border bg-card p-5 space-y-4 break-words">
      <div class="flex items-center justify-between"><h2 class="font-bold">{{ selected.order_no }} · {{ t(`invoice.${selected.status}`) }}</h2><Button variant="ghost" :disabled="busy" @click="selectedId = null">{{ t('invoice.close') }}</Button></div>
      <dl class="grid gap-3 sm:grid-cols-2 text-sm"><div><dt class="text-muted-foreground">{{ t('invoice.heading') }}</dt><dd>{{ selected.title }}（{{ t(`invoice.${selected.kind}`) }}）</dd></div><div><dt class="text-muted-foreground">{{ t('invoice.tax') }}</dt><dd>{{ selected.tax_id || '—' }}</dd></div><div><dt class="text-muted-foreground">{{ t('invoice.email') }}</dt><dd>{{ selected.email }}</dd></div><div><dt class="text-muted-foreground">{{ t('invoice.amount') }}</dt><dd>{{ selected.amount }} {{ selected.currency }}</dd></div><div class="sm:col-span-2"><dt class="text-muted-foreground">{{ t('invoice.note') }}</dt><dd class="whitespace-pre-wrap">{{ selected.note || '—' }}</dd></div></dl>
      <p v-if="selected.rejection_reason">{{ t('invoice.reason') }}：{{ selected.rejection_reason }}</p>
      <form v-if="selected.status === 'pending' && can('review')" class="space-y-3" @submit.prevent="act('review', { decision: 'reject', reason })">
        <Button type="button" :disabled="busy || loading" @click="act('review', { decision: 'approve' })">{{ t('invoice.approve') }}</Button>
        <label class="block text-sm">{{ t('invoice.reason') }}<textarea v-model="reason" required maxlength="1000" :disabled="busy" class="mt-2 block w-full rounded-md border bg-background p-3" /></label>
        <Button type="submit" variant="destructive" :disabled="busy || loading || !reason.trim()">{{ t('invoice.reject') }}</Button>
      </form>
      <form v-if="selected.status === 'approved' && can('file')" class="space-y-3" @submit.prevent="upload">
        <label class="block text-sm">{{ t('invoice.pdf') }}<input :key="selected.id" type="file" accept="application/pdf,.pdf" required :disabled="busy" class="mt-3 block w-full" @change="chooseFile" /></label>
        <Button type="submit" :disabled="busy || loading || !file">{{ t('invoice.upload') }}</Button>
      </form>
      <template v-if="selected.status === 'issued'">
        <p>{{ t('invoice.mail') }}：{{ t(`invoice.mail_${selected.mail_status}`) }}</p>
        <p v-if="selected.mail_error" class="text-destructive">{{ selected.mail_error }}</p>
        <p v-if="selected.sent_at" class="text-sm">{{ t('invoice.sentAt') }}：{{ new Date(selected.sent_at).toLocaleString() }}</p>
        <div class="flex flex-wrap gap-3"><Button v-if="auth.hasPermission('GET:/admin/invoices/:id/download')" variant="outline" :disabled="busy" @click="download">{{ t('invoice.download') }}</Button><Button v-if="selected.mail_status !== 'sent' && can('send')" :disabled="busy || loading || (selected.mail_status === 'sending' && Date.now() - new Date(selected.sending_at).getTime() < 600000)" @click="act('send')">{{ t(selected.mail_status === 'pending' ? 'invoice.send' : 'invoice.retry') }}</Button></div>
        <p class="text-sm text-muted-foreground">{{ t('invoice.refundHint') }}</p>
      </template>
    </section>
  </div>
</template>
