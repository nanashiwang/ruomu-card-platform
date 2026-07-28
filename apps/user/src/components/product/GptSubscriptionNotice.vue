<template>
  <section class="my-6 overflow-hidden rounded-xl border border-amber-300 bg-amber-50/80 text-amber-950 shadow-sm dark:border-amber-800/70 dark:bg-amber-950/30 dark:text-amber-100" role="note" aria-labelledby="gpt-user-notice-title">
    <header class="border-b border-amber-200 bg-amber-100/80 px-4 py-3 dark:border-amber-800/60 dark:bg-amber-900/40">
      <h2 id="gpt-user-notice-title" class="flex items-center gap-2 text-base font-extrabold">
        <ShieldAlert class="h-5 w-5 flex-none" aria-hidden="true" />
        {{ t('productDetail.gptNotice.title') }}
      </h2>
    </header>

    <div class="space-y-4 px-4 py-4 text-sm leading-6">
      <div>
        <h3 class="flex items-center gap-2 font-bold"><Clock3 class="h-4 w-4 flex-none" aria-hidden="true" />{{ t('productDetail.gptNotice.operationTitle') }}</h3>
        <p class="mt-1">{{ t('productDetail.gptNotice.operationText') }}</p>
        <p class="mt-1 font-semibold">{{ t('productDetail.gptNotice.businessHours') }}</p>
      </div>

      <div>
        <h3 class="flex items-center gap-2 font-bold"><MailCheck class="h-4 w-4 flex-none" aria-hidden="true" />{{ t('productDetail.gptNotice.emailTitle') }}</h3>
        <p class="mt-1">{{ t('productDetail.gptNotice.emailText') }}</p>
      </div>

      <div class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2.5 text-emerald-900 dark:border-emerald-800/70 dark:bg-emerald-950/30 dark:text-emerald-100">
        <h3 class="flex items-center gap-2 font-bold"><BellRing class="h-4 w-4 flex-none" aria-hidden="true" />{{ t('productDetail.gptNotice.completionEmailTitle') }}</h3>
        <p class="mt-1">{{ t('productDetail.gptNotice.completionEmailText') }}</p>
      </div>

      <div>
        <h3 class="flex items-center gap-2 font-bold text-red-700 dark:text-red-300"><Ban class="h-4 w-4 flex-none" aria-hidden="true" />{{ t('productDetail.gptNotice.prohibitedTitle') }}</h3>
        <p class="mt-1">{{ t('productDetail.gptNotice.prohibitedCurrent') }}</p>
        <p class="mt-1">{{ t('productDetail.gptNotice.prohibitedHistory') }}</p>
        <p class="mt-1">{{ t('productDetail.gptNotice.consequences') }}</p>
      </div>

      <p class="rounded-lg border border-amber-300 bg-white/70 px-3 py-2 font-semibold dark:border-amber-800/70 dark:bg-black/10">
        {{ t('productDetail.gptNotice.serviceScope') }}
      </p>

      <div class="border-t border-amber-200 pt-4 dark:border-amber-800/60">
        <h3 class="flex items-center gap-2 text-base font-extrabold text-red-800 dark:text-red-200">
          <CircleDollarSign class="h-5 w-5 flex-none" aria-hidden="true" />
          {{ t('productDetail.gptNotice.refundTermsTitle') }}
        </h3>
        <ol class="mt-3 space-y-3">
          <li v-for="term in refundTerms" :key="term.number" class="flex gap-3 rounded-lg border border-amber-200 bg-white/70 px-3 py-3 dark:border-amber-800/60 dark:bg-black/10">
            <span class="grid h-7 w-7 flex-none place-items-center rounded-full bg-amber-200 text-xs font-black text-amber-900 dark:bg-amber-800 dark:text-amber-100">{{ term.number }}</span>
            <div class="min-w-0">
              <h4 class="font-bold">{{ t(term.titleKey) }}</h4>
              <p class="mt-1 text-amber-900/90 dark:text-amber-100/90">{{ t(term.textKey) }}</p>
            </div>
          </li>
        </ol>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { Ban, BellRing, CircleDollarSign, Clock3, MailCheck, ShieldAlert } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const refundTerms = [
  { number: '01', titleKey: 'productDetail.gptNotice.subscriptionTermsTitle', textKey: 'productDetail.gptNotice.subscriptionTermsText' },
  { number: '02', titleKey: 'productDetail.gptNotice.violationTermsTitle', textKey: 'productDetail.gptNotice.violationTermsText' },
  { number: '03', titleKey: 'productDetail.gptNotice.accountTermsTitle', textKey: 'productDetail.gptNotice.accountTermsText' },
  { number: '04', titleKey: 'productDetail.gptNotice.officialRefundTitle', textKey: 'productDetail.gptNotice.officialRefundText' },
  { number: '05', titleKey: 'productDetail.gptNotice.refundFeeTitle', textKey: 'productDetail.gptNotice.refundFeeText' },
] as const
</script>
