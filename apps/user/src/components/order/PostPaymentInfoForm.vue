<template>
  <div v-if="item?.post_payment_info_required" class="mt-3 rounded-xl border border-amber-200 bg-amber-50/60 p-4 text-sm dark:border-amber-900/70 dark:bg-amber-950/20">
    <div class="font-semibold text-foreground">{{ t('orderDetail.postPaymentInfoTitle') }}</div>
    <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ t('orderDetail.postPaymentInfoHint') }}</p>

    <div v-if="savedInfo && !editing" class="mt-3 space-y-1.5 text-xs">
      <div><span class="text-muted-foreground">{{ t('orderDetail.contactEmailLabel') }}：</span>{{ savedInfo.contact_email }}</div>
      <div><span class="text-muted-foreground">{{ t('orderDetail.currentPlanLabel') }}：</span>{{ planLabel(savedInfo.current_plan) }}</div>
      <div><span class="text-muted-foreground">{{ t('orderDetail.orderNoteLabel') }}：</span><span class="whitespace-pre-wrap break-words">{{ savedInfo.order_note }}</span></div>
      <div class="flex flex-wrap items-center gap-2 pt-1">
        <span class="font-medium text-emerald-700 dark:text-emerald-400">{{ t('orderDetail.postPaymentInfoSubmitted') }}</span>
        <button v-if="editable" type="button" class="text-primary hover:underline" @click="editing = true">{{ t('orderDetail.postPaymentInfoEdit') }}</button>
      </div>
    </div>

    <div v-else-if="!editable" class="mt-3 text-xs font-medium text-amber-700 dark:text-amber-400">
      {{ orderStatus === 'pending_payment' ? t('orderDetail.postPaymentInfoAfterPay') : t('orderDetail.postPaymentInfoLocked') }}
    </div>

    <form v-else class="mt-3 grid gap-3" @submit.prevent="submit">
      <label class="grid gap-1.5">
        <span class="text-xs font-medium text-foreground">{{ t('orderDetail.contactEmailLabel') }}</span>
        <input v-model.trim="contactEmail" type="email" autocomplete="email" required maxlength="254"
          class="h-10 rounded-lg border bg-background px-3 text-sm outline-none focus:border-primary"
          :placeholder="t('orderDetail.contactEmailPlaceholder')" />
      </label>
      <label class="grid gap-1.5">
        <span class="text-xs font-medium text-foreground">{{ t('orderDetail.currentPlanLabel') }}</span>
        <select v-model="currentPlan" required class="h-10 rounded-lg border bg-background px-3 text-sm outline-none focus:border-primary">
          <option value="" disabled>{{ t('orderDetail.currentPlanPlaceholder') }}</option>
          <option v-for="option in planOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
      </label>
      <label class="grid gap-1.5">
        <span class="text-xs font-medium text-foreground">{{ t('orderDetail.orderNoteLabel') }}</span>
        <textarea v-model.trim="orderNote" required maxlength="1000" rows="4"
          class="min-h-24 resize-y rounded-lg border bg-background px-3 py-2 text-sm outline-none focus:border-primary"
          :placeholder="t('orderDetail.orderNotePlaceholder')"></textarea>
      </label>
      <p class="text-xs leading-5 text-muted-foreground">{{ t('orderDetail.postPaymentInfoSafety') }}</p>
      <div class="flex items-center gap-2">
        <button type="submit" :disabled="submitting" class="rounded-lg bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground disabled:opacity-60">
          {{ submitting ? t('orderDetail.postPaymentInfoSubmitting') : t('orderDetail.postPaymentInfoSubmit') }}
        </button>
        <button v-if="savedInfo" type="button" class="px-2 py-2 text-xs text-muted-foreground hover:text-foreground" @click="cancelEdit">
          {{ t('common.cancel') }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { guestOrderAPI, userOrderAPI } from '../../api'
import { toast } from '../../composables/useToast'

const props = defineProps<{
  item: any
  orderNo: string
  orderStatus: string
  guestAuth?: { email: string; order_password: string }
}>()

const { t } = useI18n()
const savedInfo = ref<any>(props.item?.post_payment_info || null)
const contactEmail = ref(savedInfo.value?.contact_email || props.guestAuth?.email || '')
const currentPlan = ref(savedInfo.value?.current_plan || '')
const orderNote = ref(savedInfo.value?.order_note || '')
const editing = ref(!savedInfo.value)
const submitting = ref(false)
const editable = computed(() => ['paid', 'fulfilling'].includes(String(props.orderStatus || '')))

const planOptions = computed(() => [
  'free', 'go', 'plus', 'pro', 'business', 'team', 'enterprise', 'edu', 'other',
].map(value => ({ value, label: t(`orderDetail.currentPlans.${value}`) })))

const planLabel = (value: string) => t(`orderDetail.currentPlans.${value || 'other'}`)

const findItem = (detail: any) => {
  const items = [...(detail?.items || []), ...(detail?.children || []).flatMap((child: any) => child?.items || [])]
  return items.find((candidate: any) => Number(candidate?.id) === Number(props.item?.id))
}

const cancelEdit = () => {
  contactEmail.value = savedInfo.value?.contact_email || props.guestAuth?.email || ''
  currentPlan.value = savedInfo.value?.current_plan || ''
  orderNote.value = savedInfo.value?.order_note || ''
  editing.value = false
}

const submit = async () => {
  if (submitting.value || !props.item?.id) return
  submitting.value = true
  try {
    const payload = { contact_email: contactEmail.value, current_plan: currentPlan.value, order_note: orderNote.value }
    const response = props.guestAuth
      ? await guestOrderAPI.submitPostPaymentInfo(props.orderNo, Number(props.item.id), {
          ...payload,
          email: props.guestAuth.email,
          order_password: props.guestAuth.order_password,
        })
      : await userOrderAPI.submitPostPaymentInfo(props.orderNo, Number(props.item.id), payload)
    const updated = findItem(response.data?.data)
    savedInfo.value = updated?.post_payment_info || {
      contact_email: contactEmail.value,
      current_plan: currentPlan.value,
      order_note: orderNote.value,
    }
    editing.value = false
    toast.success(t('orderDetail.postPaymentInfoSuccess'))
  } catch {
    toast.error(t('orderDetail.postPaymentInfoFailed'))
  } finally {
    submitting.value = false
  }
}
</script>
