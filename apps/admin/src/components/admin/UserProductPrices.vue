<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminProduct, AdminProductSKU } from '@/api/types'
import { useAdminAuthStore } from '@/stores/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { formatMoney, getLocalizedText } from '@/utils/format'
import { confirmAction } from '@/utils/confirm'
const props = defineProps<{ userId: number; currency: string }>()
const { t } = useI18n()
const auth = useAdminAuthStore()
const canWrite = computed(() => auth.hasPermission(`PUT:/admin/users/${props.userId}/product-prices`))
const canDelete = computed(() => auth.hasPermission(`DELETE:/admin/users/${props.userId}/product-prices/1`))
interface Price { id: number; product_id: number; sku_id: number; price_amount: string; product?: AdminProduct; sku?: AdminProductSKU }
const prices = ref<Price[]>([])
const products = ref<AdminProduct[]>([])
const selected = ref<AdminProduct | null>(null)
const productOptions = computed(() => selected.value && !products.value.some(p => p.id === selected.value!.id) ? [selected.value, ...products.value] : products.value)
const productId = ref('')
const skuId = ref('0')
const amount = ref('')
const search = ref('')
const page = ref(1)
const totalPages = ref(1)
const loading = ref(false)
const searching = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
let searchVersion = 0
let selectionVersion = 0
const skuLabel = (sku: AdminProductSKU) => Object.values(sku.spec_values || {}).join(' / ') || sku.sku_code || `#${sku.id}`
async function load() {
 loading.value = true; error.value = ''
 try { prices.value = (await adminAPI.getUserProductPrices(props.userId)).data.data || [] }
 catch (e: any) { error.value = e?.message || t('userPrices.failed') }
 finally { loading.value = false }
}
async function findProducts(nextPage = 1) {
 const version = ++searchVersion
 searching.value = true; error.value = ''
 try {
  const result = await adminAPI.getProducts({ search: search.value.trim(), page: nextPage, page_size: 20 })
  if (version !== searchVersion) return
  products.value = result.data.data || []; page.value = nextPage; totalPages.value = result.data.pagination?.total_page || 1
 } catch (e: any) { if (version === searchVersion) error.value = e?.message || t('userPrices.failed') }
 finally { if (version === searchVersion) searching.value = false }
}
async function chooseProduct(id: number, editSKU = 0, editAmount = '') {
 const version = ++selectionVersion
 selected.value = null; skuId.value = String(editSKU); amount.value = editAmount; productId.value = String(id)
 if (!id) return
 error.value = ''
 try {
  const response = await adminAPI.getProduct(id)
  if (version !== selectionVersion) return
  selected.value = response.data.data
  if (selected.value && !products.value.some(p => p.id === id)) products.value.unshift(selected.value)
 } catch (e: any) { if (version === selectionVersion) error.value = e?.message || t('userPrices.failed') }
}
function reset() { ++selectionVersion; selected.value = null; productId.value = ''; skuId.value = '0'; amount.value = '' }
async function save() {
 if (!selected.value || !/^\d+(\.\d{1,2})?$/.test(amount.value) || Number(amount.value) <= 0) { error.value = t('userPrices.invalid'); return }
 saving.value = true; error.value = ''; notice.value = ''
 try {
  await adminAPI.setUserProductPrice(props.userId, { product_id: selected.value.id, sku_id: Number(skuId.value), price_amount: amount.value })
  notice.value = t('userPrices.saved'); reset(); await load()
 } catch (e: any) { error.value = e?.message || t('userPrices.failed') }
 finally { saving.value = false }
}
async function remove(row: Price) {
 if (!await confirmAction(t('userPrices.confirm'))) return
 saving.value = true; error.value = ''; notice.value = ''
 try { await adminAPI.deleteUserProductPrice(props.userId, row.id); notice.value = t('userPrices.removed'); await load() }
 catch (e: any) { error.value = e?.message || t('userPrices.failed') }
 finally { saving.value = false }
}
onMounted(() => { void load(); if (canWrite.value) void findProducts() })
</script>
<template>
 <section class="space-y-4">
  <p class="text-sm text-muted-foreground">{{ t('userPrices.hint') }}</p>
  <div v-if="error" role="alert" class="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{{ error }} <Button variant="outline" size="sm" :disabled="loading || saving" @click="load">{{ t('userPrices.retry') }}</Button></div>
  <p v-if="notice" role="status" class="text-sm text-emerald-600">{{ notice }}</p>
  <div v-if="canWrite" class="rounded-xl border border-border bg-card p-4 space-y-4">
   <form class="flex gap-2" @submit.prevent="findProducts()">
    <Input v-model="search" :aria-label="t('userPrices.search')" :placeholder="t('userPrices.search')" :disabled="saving" />
    <Button variant="outline" type="submit" :disabled="searching || saving">{{ t('userPrices.searchButton') }}</Button>
   </form>
   <form class="grid gap-4 md:grid-cols-3" @submit.prevent="save">
    <label class="space-y-2 text-sm">{{ t('userPrices.product') }}
     <select v-model="productId" class="block w-full rounded-md border border-input bg-background p-2" :disabled="searching || saving" @change="chooseProduct(Number(productId))">
      <option value="">{{ t('userPrices.choose') }}</option>
      <option v-for="p in productOptions" :key="p.id" :value="String(p.id)">#{{ p.id }} {{ getLocalizedText(p.title) }}</option>
     </select>
    </label>
    <label class="space-y-2 text-sm">{{ t('userPrices.sku') }}
     <select v-model="skuId" class="block w-full rounded-md border border-input bg-background p-2" :disabled="!selected || saving">
      <option value="0">{{ t('userPrices.all') }}</option>
      <option v-for="sku in selected?.skus || []" :key="sku.id" :value="String(sku.id)">{{ skuLabel(sku) }}</option>
     </select>
    </label>
    <label class="space-y-2 text-sm">{{ t('userPrices.price') }} ({{ currency }})<Input v-model="amount" inputmode="decimal" maxlength="21" placeholder="0.00" :disabled="saving" /></label>
    <div class="flex flex-wrap gap-2 md:col-span-3">
     <Button type="submit" :disabled="saving || !selected">{{ t('userPrices.save') }}</Button>
     <Button type="button" variant="outline" :disabled="saving" @click="reset">{{ t('userPrices.cancel') }}</Button>
     <Button type="button" variant="ghost" :disabled="searching || saving || page <= 1" @click="findProducts(page - 1)">{{ t('userPrices.previous') }}</Button>
     <Button type="button" variant="ghost" :disabled="searching || saving || page >= totalPages" @click="findProducts(page + 1)">{{ t('userPrices.next') }}</Button>
    </div>
    <p v-if="!searching && !products.length" class="text-sm text-muted-foreground md:col-span-3">{{ t('userPrices.noProducts') }}</p>
   </form>
  </div>
  <p v-else class="text-sm text-muted-foreground">{{ t('userPrices.readonly') }}</p>
  <p v-if="loading">{{ t('userPrices.loading') }}</p>
  <p v-else-if="!prices.length && !error" class="rounded-xl border p-8 text-center text-muted-foreground">{{ t('userPrices.empty') }}</p>
  <div v-else class="overflow-x-auto rounded-xl border border-border bg-card">
   <table class="w-full min-w-[640px] text-sm"><thead class="bg-muted/40"><tr class="text-left"><th class="p-4">{{ t('userPrices.product') }}</th><th class="p-4">{{ t('userPrices.sku') }}</th><th class="p-4">{{ t('userPrices.price') }}</th><th class="p-4"></th></tr></thead>
    <tbody><tr v-for="row in prices" :key="row.id" class="border-t border-border">
     <td class="p-4">#{{ row.product_id }} {{ row.product ? getLocalizedText(row.product.title) : t('userPrices.deleted') }}</td>
     <td class="p-4">{{ row.sku_id ? (row.sku ? skuLabel(row.sku) : `#${row.sku_id} · ${t('userPrices.deleted')}`) : t('userPrices.all') }}</td>
     <td class="p-4 whitespace-nowrap">{{ formatMoney(row.price_amount, currency) }}</td>
     <td class="p-4"><div class="flex gap-2"><Button v-if="canWrite && row.product" variant="outline" size="sm" :disabled="saving" @click="chooseProduct(row.product_id, row.sku_id, row.price_amount)">{{ t('userPrices.edit') }}</Button><Button v-if="canDelete" variant="outline" size="sm" :disabled="saving" @click="remove(row)">{{ t('userPrices.remove') }}</Button></div></td>
    </tr></tbody>
   </table>
  </div>
 </section>
</template>
