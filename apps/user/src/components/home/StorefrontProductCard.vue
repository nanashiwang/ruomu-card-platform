<template>
  <article class="offer-card" :class="{ 'is-sold-out': isSoldOut(product) }">
    <div class="offer-top">
      <div class="offer-icon">
        <img v-if="imageSrc && !imageFailed" :src="imageSrc" alt="" loading="lazy" @error="imageFailed = true" />
        <Package v-else aria-hidden="true" />
      </div>
      <span class="stock-label" :class="{ unavailable: isSoldOut(product) }">{{ getStockStatusLabel(product) }}</span>
    </div>
    <RouterLink :to="{ name: 'product-detail', params: { slug: product.slug } }" class="offer-title"><h3>{{ getLocalizedText(product.title) }}</h3></RouterLink>
    <p class="offer-description">{{ getLocalizedText(product.description) }}</p>
    <div class="offer-price">
      <span class="price-label">{{ t('storefront.from') }}</span>
      <strong>{{ formatPrice(hasPromotionPrice(product) ? getPromotionPriceAmount(product) : product.price_amount, siteCurrency) }}</strong>
      <span v-if="hasPromotionPrice(product)" class="promotion"><s>{{ formatPrice(product.price_amount, siteCurrency) }}</s> · {{ t('products.promotionTag') }}</span>
      <span v-else-if="hasWholesalePrices(product)" class="promotion">{{ t('products.wholesaleTag') }}</span>
      <span v-else-if="hasPromotionRules(product)" class="promotion">{{ t('products.promotionBadge') }}</span>
    </div>
    <div class="offer-facts">
      <span><Check aria-hidden="true" />{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
      <span><Check aria-hidden="true" />{{ getPurchaseTypeLabel(product.purchase_type) }}</span>
      <span v-for="tag in (product.tags || []).slice(0, 2)" :key="tag"><Check aria-hidden="true" />{{ tag }}</span>
    </div>
    <button type="button" class="offer-buy" :disabled="isSoldOut(product)" @click="$emit('quickBuy', product)">
      {{ isSoldOut(product) ? t('products.stockStatus.outOfStock') : t('storefront.buy') }}<ArrowRight aria-hidden="true" />
    </button>
    <RouterLink class="offer-details" :to="{ name: 'product-detail', params: { slug: product.slug } }">{{ t('storefront.details') }}<ChevronRight aria-hidden="true" /></RouterLink>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, Check, ChevronRight, Package } from 'lucide-vue-next'
import { useLocalized, useProductLabels } from '../../composables/useProduct'
import { getFirstImageUrl, getImageUrl } from '../../utils/image'
const props = defineProps<{ product: any }>()
defineEmits<{ quickBuy: [product: any] }>()
const { t } = useI18n()
const { getLocalizedText, formatPrice, siteCurrency } = useLocalized()
const { isSoldOut, getStockStatusLabel, getPurchaseTypeLabel, getFulfillmentTypeLabel, hasPromotionPrice, getPromotionPriceAmount, hasWholesalePrices, hasPromotionRules } = useProductLabels()
const imageSrc = computed(() => getFirstImageUrl(props.product.images) || getImageUrl(props.product.category?.icon || ''))
const imageFailed = ref(false)
watch(imageSrc, () => { imageFailed.value = false })
</script>

<style scoped>
.offer-card { display: flex; flex-direction: column; min-width: 0; padding: 28px; border: 1px solid var(--landing-line); border-radius: 22px; background: var(--landing-card); transition: border-color .2s, box-shadow .2s, transform .2s; }
.offer-card:hover { border-color: var(--landing-accent); box-shadow: 0 16px 36px #1725540c; transform: translateY(-4px); }
.offer-top { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 22px; }
.offer-icon { display: grid; place-items: center; width: 52px; height: 52px; border-radius: 15px; overflow: hidden; background: var(--landing-soft); color: var(--landing-accent); }
.offer-icon img { width: 100%; height: 100%; object-fit: cover; }
.stock-label { font-size: 12px; color: var(--landing-accent); background: var(--landing-soft); padding: 5px 10px; border-radius: 50px; }
.stock-label.unavailable { color: var(--ui-danger); background: var(--ui-danger-soft); }
.offer-title { color: inherit; text-decoration: none; }
h3 { font-size: 21px; line-height: 1.4; font-weight: 750; overflow-wrap: anywhere; }
.offer-description { font-size: 13px; color: var(--landing-muted); line-height: 1.8; margin-top: 10px; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; min-height: 47px; }
.offer-price { display: flex; flex-direction: column; gap: 5px; padding: 24px 0; }
.price-label { color: var(--landing-muted); font-size: 11px; }
.offer-price strong { font-size: 27px; letter-spacing: -.04em; line-height: 1.3; overflow-wrap: anywhere; }
.promotion { color: var(--landing-muted); font-size: 12px; }
.offer-facts { display: flex; flex-direction: column; gap: 11px; border-top: 1px solid var(--landing-line); padding: 21px 0 28px; font-size: 13px; }
.offer-facts span { display: flex; align-items: center; gap: 9px; }
.offer-facts svg { width: 15px; height: 15px; flex-shrink: 0; color: var(--landing-accent); }
.offer-buy { display: flex; align-items: center; justify-content: center; gap: 10px; border-radius: 11px; background: var(--landing-accent); color: white; min-height: 46px; font-weight: 650; font-size: 14px; margin-top: auto; cursor: pointer; }
.offer-buy:disabled { background: var(--landing-soft); color: var(--landing-muted); cursor: not-allowed; }
.offer-buy svg, .offer-details svg { width: 16px; height: 16px; }
.offer-details { display: flex; align-items: center; justify-content: center; gap: 3px; font-size: 12px; color: var(--landing-muted); margin-top: 14px; }
:is(a, button):focus-visible { outline: 3px solid var(--landing-accent); outline-offset: 5px; }
@media (prefers-reduced-motion: reduce) { .offer-card { transition: none; } .offer-card:hover { transform: none; } }
</style>
