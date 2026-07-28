<template>
  <aside class="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3.5 text-amber-950 shadow-sm dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-100" role="note">
    <span class="mt-0.5 grid h-9 w-9 flex-none place-items-center rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/60 dark:text-amber-300">
      <Clock3 class="h-4.5 w-4.5" aria-hidden="true" />
    </span>
    <div class="min-w-0">
      <p class="text-xs font-bold uppercase tracking-wider text-amber-700 dark:text-amber-300">{{ t('home.businessHours.label') }}</p>
      <p class="mt-0.5 text-base font-extrabold">{{ t('home.businessHours.hours') }}</p>
      <p class="mt-1 text-sm leading-6 text-amber-800/90 dark:text-amber-200/90">{{ t('home.businessHours.detail') }}</p>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { Clock3 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>
