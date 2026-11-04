<template>
  <div class="storefront-home" :class="{ 'classic-home': !isVault }">
    <section class="landing-hero" aria-labelledby="landing-title">
      <div class="landing-wrap hero-inner">
        <div class="hero-eyebrow"><span></span>{{ brandName }}<span class="eyebrow-divider">/</span>{{ t('storefront.eyebrow') }}</div>
        <h1 id="landing-title">{{ t('storefront.title') }}<br /><span>{{ t('storefront.highlight') }}</span></h1>
        <p class="hero-description">{{ brandDescription || t('storefront.description') }}</p>
        <div class="hero-actions">
          <a href="#selection" class="landing-button primary">{{ t('storefront.browse') }}<ArrowUpRight aria-hidden="true" /></a>
          <a href="#how-it-works" class="landing-button secondary"><CirclePlay aria-hidden="true" />{{ t('storefront.how') }}</a>
        </div>
        <p class="hero-promise"><ShieldCheck aria-hidden="true" />{{ t('storefront.promise') }}</p>
        <div class="hero-benefits">
          <div v-for="benefit in benefits" :key="benefit.key" class="hero-benefit">
            <span class="benefit-icon"><component :is="benefit.icon" aria-hidden="true" /></span>
            <div><h2>{{ t(`storefront.benefits.${benefit.key}.title`) }}</h2><p>{{ t(`storefront.benefits.${benefit.key}.body`) }}</p></div>
          </div>
        </div>
        <a class="hero-scroll" href="#selection" :aria-label="t('storefront.browse')"><ArrowDown aria-hidden="true" /></a>
      </div>
    </section>

    <section id="selection" class="landing-section landing-wrap" aria-labelledby="selection-title">
      <div class="section-heading">
        <div><p class="section-eyebrow">{{ t('storefront.productsLabel') }}</p><h2 id="selection-title">{{ t('storefront.productsTitle') }}</h2><p>{{ t('storefront.productsDescription') }}</p></div>
        <RouterLink class="text-link" to="/products">{{ t('home.featured.viewAll') }}<ArrowRight aria-hidden="true" /></RouterLink>
      </div>
      <nav v-if="categories.length" class="category-links" :aria-label="t('products.allCategories')">
        <RouterLink class="category-all" to="/products"><LayoutGrid aria-hidden="true" />{{ t('products.allCategories') }}</RouterLink>
        <RouterLink v-for="category in categories" :key="category.id" :to="{ name: 'category-products', params: { slug: category.slug } }">{{ getLocalizedText(category.name) }}<ArrowUpRight aria-hidden="true" /></RouterLink>
      </nav>
      <div v-if="productsLoading" class="offer-grid" :aria-label="t('common.loading')" aria-busy="true">
        <div v-for="n in 6" :key="n" class="offer-skeleton"><span></span><span></span><span></span></div>
      </div>
      <div v-else-if="productsError" class="landing-empty" role="alert"><PackageOpen aria-hidden="true" /><p>{{ t('storefront.loadError') }}</p><button class="landing-button secondary" @click="loadProducts">{{ t('errorBoundary.retry') }}</button></div>
      <div v-else-if="products.length" class="offer-grid">
        <StorefrontProductCard v-for="product in products" :key="product.id" :product="product" @quick-buy="openQuickBuy" />
      </div>
      <div v-else class="landing-empty"><PackageOpen aria-hidden="true" /><p>{{ t('products.empty') }}</p><RouterLink class="text-link" to="/products">{{ t('home.featured.viewAll') }}<ArrowRight aria-hidden="true" /></RouterLink></div>
    </section>

    <section v-if="bannerCount" class="landing-wrap campaign-section" :aria-label="t('storefront.campaign')">
      <div class="campaign" @touchstart="onBannerTouchStart" @touchend="onBannerTouchEnd">
        <img v-if="heroImage" :src="heroImage" :alt="heroTitle" loading="lazy" />
        <div class="campaign-content"><p>{{ t('storefront.campaign') }}</p><h2>{{ heroTitle }}</h2><p>{{ heroSubtitle }}</p><button v-if="hasHeroLink" class="landing-button secondary" @click="goToHeroLink">{{ heroPrimaryButtonText }}<ArrowUpRight aria-hidden="true" /></button></div>
        <div v-if="bannerCount > 1" class="campaign-controls"><button :aria-label="t('common.previousBanner')" @click="handlePrevHeroBanner"><ChevronLeft /></button><span>{{ currentBannerIndex + 1 }} / {{ bannerCount }}</span><button :aria-label="t('common.nextBanner')" @click="handleNextHeroBanner"><ChevronRight /></button></div>
      </div>
    </section>

    <section id="how-it-works" class="process-section" aria-labelledby="process-title">
      <div class="landing-wrap landing-section">
        <div class="section-heading centered"><p class="section-eyebrow">{{ t('storefront.processLabel') }}</p><h2 id="process-title">{{ t('storefront.processTitle') }}</h2><p>{{ t('storefront.processDescription') }}</p></div>
        <ol class="steps-grid">
          <li v-for="(step, index) in steps" :key="step.key"><span class="step-number">0{{ index + 1 }}</span><component :is="step.icon" class="step-icon" aria-hidden="true" /><h3>{{ t(`storefront.steps.${step.key}.title`) }}</h3><p>{{ t(`storefront.steps.${step.key}.body`) }}</p></li>
        </ol>
        <div class="order-callout"><div class="order-symbol"><ClipboardCheck aria-hidden="true" /></div><div><h3>{{ t('storefront.orderCard') }}</h3><p>{{ t('storefront.orderDescription') }}</p></div><RouterLink :to="ordersPath" class="landing-button secondary">{{ t('storefront.orders') }}<ArrowRight aria-hidden="true" /></RouterLink></div>
        <BusinessHoursNotice class="business-notice" />
      </div>
    </section>

    <section class="landing-section landing-wrap faq-section" aria-labelledby="faq-title">
      <div class="faq-intro"><p class="section-eyebrow">{{ t('storefront.faqLabel') }}</p><h2 id="faq-title">{{ t('storefront.faqTitle') }}</h2><RouterLink class="text-link" to="/terms">{{ t('footer.terms') }}<ArrowUpRight aria-hidden="true" /></RouterLink></div>
      <div class="faq-list"><details v-for="key in faqKeys" :key="key"><summary>{{ t(`storefront.faq.${key}.q`) }}<Plus aria-hidden="true" /></summary><p>{{ t(`storefront.faq.${key}.a`) }}</p></details></div>
    </section>

    <section v-if="latestVisible" class="landing-wrap news-section" aria-labelledby="news-title">
      <div class="section-heading"><div><p class="section-eyebrow">{{ t('storefront.latestLabel') }}</p><h2 id="news-title">{{ t('storefront.latestTitle') }}</h2><p>{{ t('storefront.latestDescription') }}</p></div><div class="news-links"><RouterLink v-if="blogEnabled" class="text-link" to="/blog">{{ t('nav.blog') }}<ArrowUpRight aria-hidden="true" /></RouterLink><RouterLink v-if="noticeEnabled" class="text-link" to="/notice">{{ t('nav.notice') }}<ArrowUpRight aria-hidden="true" /></RouterLink></div></div>
      <p v-if="postsLoading" class="landing-empty" role="status">{{ t('common.loading') }}</p>
      <div v-else-if="postsError" class="landing-empty" role="alert"><p>{{ t('storefront.postsError') }}</p><button class="text-link" @click="loadPosts">{{ t('errorBoundary.retry') }}</button></div>
      <div v-else-if="posts.length" class="news-grid"><RouterLink v-for="post in posts" :key="post.id" :to="{ name: 'blog-detail', params: { slug: post.slug } }" class="news-card"><div class="news-meta"><span>{{ post.type === 'notice' ? t('nav.notice') : t('nav.blog') }}</span><time v-if="post.published_at" :datetime="post.published_at">{{ formatDate(post.published_at) }}</time></div><h3>{{ getLocalizedText(post.title) }}</h3><p>{{ getLocalizedText(post.summary) }}</p><span class="text-link">{{ t('blog.readMore') }}<ArrowRight aria-hidden="true" /></span></RouterLink></div>
      <p v-else class="landing-empty">{{ t('blog.empty') }}</p>
    </section>

    <section class="closing-section"><div class="landing-wrap"><Sparkles aria-hidden="true" /><h2>{{ t('storefront.closingTitle') }}</h2><p>{{ t('storefront.closingDescription') }}</p><RouterLink class="landing-button primary" to="/products">{{ t('storefront.browse') }}<ArrowUpRight aria-hidden="true" /></RouterLink></div></section>
    <ProductQuickBuy v-if="quickBuyProduct" :product="quickBuyProduct" :visible="quickBuyVisible" @update:visible="quickBuyVisible = $event" />
    <AnnouncementModal v-if="activeAnnouncement" :announcement="activeAnnouncement" :visible="announcementVisible" @update:visible="announcementVisible = $event" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, ArrowRight, ArrowUpRight, CheckCheck, ChevronLeft, ChevronRight, CirclePlay, ClipboardCheck, CreditCard, LayoutGrid, PackageCheck, PackageOpen, Plus, ShieldCheck, SlidersHorizontal, Sparkles } from 'lucide-vue-next'
import { categoryAPI, postAPI, productAPI } from '../api'
import { useAppStore } from '../stores/app'
import { useUserAuthStore } from '../stores/userAuth'
import { getActiveTemplate } from '../templates/registry'
import { useLocalized, useProductLabels } from '../composables/useProduct'
import { usePageSeo } from '../composables/usePageSeo'
import { useBannerCarousel } from '../composables/useBannerCarousel'
import { useAnnouncement, type HomeAnnouncement } from '../composables/useAnnouncement'
import { buildCategoryGroups, type PublicCategory } from '../utils/category'
import StorefrontProductCard from '../components/home/StorefrontProductCard.vue'
import ProductQuickBuy from '../components/ProductQuickBuy.vue'
import AnnouncementModal from '../components/AnnouncementModal.vue'
import BusinessHoursNotice from '../components/BusinessHoursNotice.vue'

const { t } = useI18n()
const appStore = useAppStore()
const userAuth = useUserAuthStore()
const { getLocalizedText } = useLocalized()
const { isSoldOut } = useProductLabels()
const isVault = computed(() => getActiveTemplate() === 'vault')
const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || t('common.siteName'))
const brandDescription = computed(() => getLocalizedText(appStore.config?.brand?.site_description))
const ordersPath = computed(() => userAuth.isAuthenticated ? '/me/orders' : '/guest/orders')
const builtin = computed(() => appStore.config?.nav_config?.builtin)
const blogEnabled = computed(() => builtin.value?.blog !== false)
const noticeEnabled = computed(() => builtin.value?.notice !== false)
const latestVisible = computed(() => blogEnabled.value || noticeEnabled.value)
const benefits = [{ key: 'choice', icon: SlidersHorizontal }, { key: 'delivery', icon: PackageCheck }, { key: 'tracking', icon: CheckCheck }]
const steps = [{ key: 'choose', icon: LayoutGrid }, { key: 'pay', icon: CreditCard }, { key: 'receive', icon: PackageCheck }]
const faqKeys = ['account', 'payment', 'delivery', 'lookup', 'support']
const products = ref<any[]>([])
const productsLoading = ref(true)
const productsError = ref(false)
const categories = ref<PublicCategory[]>([])
const posts = ref<any[]>([])
const postsLoading = ref(true)
const postsError = ref(false)
const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)
const openQuickBuy = (product: any) => {
  if (isSoldOut(product)) return
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}
const { shouldShow } = useAnnouncement()
const activeAnnouncement = ref<HomeAnnouncement | null>(null)
const announcementVisible = ref(false)
const { bannerCount, currentBannerIndex, heroImage, heroTitle, heroSubtitle, hasHeroLink, heroPrimaryButtonText, loadBanners, goToHeroLink, handlePrevHeroBanner, handleNextHeroBanner, onBannerTouchStart, onBannerTouchEnd, stopHeroAutoPlay } = useBannerCarousel()
usePageSeo({ canonicalPath: () => '/' })

const loadProducts = async () => {
  productsLoading.value = true
  productsError.value = false
  try {
    const response = await productAPI.list({ page: 1, page_size: 6 })
    products.value = response.data.data || []
  } catch {
    productsError.value = true
  } finally {
    productsLoading.value = false
  }
}
const loadCategories = async () => {
  try {
    const response = await categoryAPI.list()
    categories.value = buildCategoryGroups(response.data.data || []).filter(category => category.slug)
  } catch { categories.value = [] }
}
const loadPosts = async () => {
  if (!latestVisible.value) { postsLoading.value = false; return }
  postsLoading.value = true
  postsError.value = false
  try {
    const params: Record<string, unknown> = { page: 1, page_size: 3 }
    if (!blogEnabled.value) params.type = 'notice'
    if (!noticeEnabled.value) params.type = 'blog'
    const response = await postAPI.list(params)
    posts.value = response.data.data || []
  } catch { postsError.value = true }
  finally { postsLoading.value = false }
}
const formatDate = (date: string) => new Date(date).toLocaleDateString(appStore.locale)
let disposed = false
onMounted(async () => {
  const announcement = appStore.config?.announcement as HomeAnnouncement | undefined
  if (announcement && shouldShow(announcement)) {
    activeAnnouncement.value = announcement
    announcementVisible.value = true
  }
  await Promise.all([loadProducts(), loadCategories(), loadPosts(), loadBanners()])
  // Navigation may finish before the banner request; do not leave its timer running.
  if (disposed) stopHeroAutoPlay()
})
onUnmounted(() => { disposed = true; stopHeroAutoPlay() })
</script>

<style scoped>
.storefront-home { --landing-bg: #fcfcfe; --landing-card: #fff; --landing-ink: #171c2d; --landing-muted: #697386; --landing-line: #e6e9f0; --landing-accent: #5750dc; --landing-soft: #eeedfd; color: var(--landing-ink); background: var(--landing-bg); font-family: Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; }
:global(.dark .storefront-home) { --landing-bg: #11131b; --landing-card: #1b1e29; --landing-ink: #f1f3fa; --landing-muted: #acb5c8; --landing-line: #303648; --landing-accent: #8178fa; --landing-soft: #27243f; }
.classic-home { padding-top: 74px; }
.landing-wrap { max-width: 1160px; margin: 0 auto; padding-inline: 28px; }
.landing-hero { background: radial-gradient(ellipse at 50% 0%, #e9e9ff77, transparent 65%); text-align: center; border-bottom: 1px solid var(--landing-line); }
.hero-inner { padding-top: 79px; padding-bottom: 29px; }
.hero-eyebrow { display: inline-flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: 10px; font-size: 12px; color: var(--landing-muted); margin-bottom: 26px; }
.hero-eyebrow > span:first-child { width: 7px; height: 7px; border-radius: 50%; background: #16a286; box-shadow: 0 0 0 4px #16a28613; }
.eyebrow-divider { opacity: .35; }
h1 { margin: 0; font-size: clamp(38px, 5.1vw, 72px); font-weight: 800; line-height: 1.3; letter-spacing: -.055em; }
h1 > span { color: var(--landing-accent); }
.hero-description { max-width: 600px; margin: 26px auto 0; font-size: 16px; line-height: 1.9; color: var(--landing-muted); }
.hero-actions { display: flex; justify-content: center; flex-wrap: wrap; gap: 14px; margin-top: 31px; }
.landing-button { display: inline-flex; align-items: center; justify-content: center; gap: 10px; min-height: 48px; padding: 12px 24px; border-radius: 11px; font-size: 14px; font-weight: 650; line-height: 1.5; cursor: pointer; transition: box-shadow .2s, transform .2s; }
.landing-button svg { width: 18px; height: 18px; flex-shrink: 0; }
.primary { background: linear-gradient(110deg, #4c55db, #7951e6); color: #fff; box-shadow: 0 5px 14px #6153d929; border: 1px solid transparent; }
.secondary { background: var(--landing-card); border: 1px solid var(--landing-line); color: var(--landing-ink); }
.landing-button:hover { transform: translateY(-2px); box-shadow: 0 6px 22px #4c55db1c; }
.hero-promise { display: flex; align-items: center; justify-content: center; gap: 6px; color: var(--landing-muted); margin-top: 21px; font-size: 11px; }
.hero-promise svg { width: 14px; height: 14px; flex-shrink: 0; }
.hero-benefits { display: grid; grid-template-columns: repeat(3, 1fr); max-width: 920px; margin: 55px auto 0; text-align: left; }
.hero-benefit { display: flex; align-items: center; gap: 13px; padding: 0 22px; }
.hero-benefit + .hero-benefit { border-left: 1px solid var(--landing-line); }
.benefit-icon { display: grid; place-items: center; width: 40px; height: 40px; background: var(--landing-soft); border-radius: 12px; color: var(--landing-accent); flex-shrink: 0; }
.benefit-icon svg { width: 20px; height: 20px; }
.hero-benefit h2 { font-size: 13px; font-weight: 700; margin-bottom: 5px; }
.hero-benefit p { color: var(--landing-muted); font-size: 11px; line-height: 1.6; }
.hero-scroll { display: inline-grid; place-items: center; width: 32px; height: 32px; color: var(--landing-muted); margin-top: 33px; }
.hero-scroll svg { width: 18px; height: 18px; }
.landing-section { padding-block: 86px; scroll-margin-top: 100px; }
.section-heading { display: flex; justify-content: space-between; align-items: flex-end; gap: 24px; margin-bottom: 32px; }
.section-eyebrow { color: var(--landing-accent); font-size: 12px; font-weight: 650; letter-spacing: .08em; margin-bottom: 12px; }
.section-heading h2, .faq-intro h2, .closing-section h2 { font-size: clamp(27px, 3vw, 36px); font-weight: 750; letter-spacing: -.045em; line-height: 1.4; }
.section-heading p:not(.section-eyebrow) { color: var(--landing-muted); font-size: 14px; margin-top: 12px; line-height: 1.8; }
.text-link { display: inline-flex; align-items: center; gap: 7px; color: var(--landing-accent); font-size: 13px; font-weight: 600; flex-shrink: 0; }
.text-link svg { width: 16px; height: 16px; }
.category-links { display: flex; gap: 9px; flex-wrap: wrap; margin-bottom: 25px; }
.category-links a { display: inline-flex; align-items: center; gap: 7px; padding: 8px 14px; border: 1px solid var(--landing-line); border-radius: 40px; font-size: 12px; color: var(--landing-muted); background: var(--landing-card); }
.category-links a:hover { border-color: var(--landing-accent); color: var(--landing-accent); }
.category-links .category-all { background: var(--landing-ink); color: var(--landing-bg); border-color: transparent; }
.category-links svg { width: 13px; height: 13px; }
.offer-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 22px; }
.offer-skeleton { height: 430px; border: 1px solid var(--landing-line); border-radius: 22px; padding: 28px; background: var(--landing-card); }
.offer-skeleton span { display: block; height: 25px; margin-bottom: 30px; background: var(--landing-soft); border-radius: 8px; }
.offer-skeleton span:first-child { width: 52px; height: 52px; }
.offer-skeleton span:last-child { width: 70%; }
.landing-empty { display: flex; align-items: center; flex-direction: column; gap: 20px; padding: 48px 24px; border: 1px dashed var(--landing-line); border-radius: 18px; color: var(--landing-muted); text-align: center; }
.campaign-section { padding-bottom: 70px; }
.campaign { position: relative; overflow: hidden; padding: 32px; border-radius: 20px; background: #252944; color: white; }
.campaign > img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; opacity: .3; }
.campaign-content { position: relative; display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
.campaign-content h2 { font-size: 24px; font-weight: 700; }
.campaign-content p { font-size: 13px; line-height: 1.8; }
.campaign-content .landing-button { margin-top: 9px; }
.campaign-controls { position: relative; display: flex; align-items: center; justify-content: flex-end; gap: 13px; font-size: 12px; margin-top: 16px; }
.campaign-controls button { padding: 6px; border: 1px solid #ffffff66; border-radius: 50%; }
.process-section { background: var(--landing-card); border-block: 1px solid var(--landing-line); scroll-margin-top: 80px; }
.centered { display: block; text-align: center; }
.steps-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 52px; margin: 46px 0 42px; list-style: none; }
.steps-grid li { position: relative; padding: 4px 12px 0; }
.step-number { display: block; color: var(--landing-line); font-size: 64px; font-weight: 800; letter-spacing: -.05em; line-height: 1; margin-bottom: 23px; }
.step-icon { position: absolute; top: 22px; left: 66px; width: 25px; height: 25px; color: var(--landing-accent); }
.steps-grid h3 { font-size: 18px; font-weight: 700; margin-bottom: 12px; }
.steps-grid p { color: var(--landing-muted); font-size: 13px; line-height: 1.9; }
.order-callout { display: flex; align-items: center; gap: 20px; padding: 25px 28px; background: var(--landing-bg); border: 1px solid var(--landing-line); border-radius: 16px; }
.order-symbol { display: grid; place-items: center; width: 46px; height: 46px; color: var(--landing-accent); background: var(--landing-soft); border-radius: 13px; flex-shrink: 0; }
.order-callout h3 { font-size: 15px; font-weight: 700; }
.order-callout p { font-size: 12px; line-height: 1.8; color: var(--landing-muted); margin-top: 7px; }
.order-callout .landing-button { flex-shrink: 0; margin-left: auto; }
.business-notice { margin-top: 20px; }
.faq-section { display: grid; grid-template-columns: 1fr 1.8fr; gap: 80px; }
.faq-intro .text-link { margin-top: 24px; }
.faq-list { border-top: 1px solid var(--landing-line); }
.faq-list details { border-bottom: 1px solid var(--landing-line); }
.faq-list summary { display: flex; align-items: center; justify-content: space-between; gap: 22px; padding: 23px 0; font-size: 15px; font-weight: 600; cursor: pointer; list-style: none; }
.faq-list summary::-webkit-details-marker { display: none; }
.faq-list summary svg { width: 18px; height: 18px; color: var(--landing-muted); flex-shrink: 0; }
.faq-list details[open] summary svg { transform: rotate(45deg); }
.faq-list details p { color: var(--landing-muted); font-size: 14px; line-height: 1.9; padding: 0 28px 24px 0; }
.news-section { padding-bottom: 85px; }
.news-links { display: flex; gap: 18px; }
.news-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 22px; }
.news-card { display: flex; flex-direction: column; padding: 26px; border-radius: 16px; border: 1px solid var(--landing-line); background: var(--landing-card); }
.news-card:hover { border-color: var(--landing-accent); }
.news-meta { display: flex; justify-content: space-between; gap: 10px; font-size: 11px; color: var(--landing-muted); }
.news-meta > span { color: var(--landing-accent); }
.news-card h3 { margin: 20px 0 12px; font-size: 18px; line-height: 1.6; font-weight: 700; }
.news-card p { color: var(--landing-muted); font-size: 13px; line-height: 1.8; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; margin-bottom: 24px; }
.news-card .text-link { margin-top: auto; }
.closing-section { text-align: center; padding: 70px 0 78px; background: var(--landing-soft); }
.closing-section > div > svg { width: 26px; height: 26px; color: var(--landing-accent); margin: 0 auto 22px; }
.closing-section p { color: var(--landing-muted); font-size: 14px; line-height: 1.8; margin: 17px 0 27px; }
:is(a, button, summary):focus-visible { outline: 3px solid var(--landing-accent); outline-offset: 5px; }
@media (min-width: 1500px) { .hero-inner { padding-top: 95px; padding-bottom: 35px; } }
@media (max-width: 900px) { .hero-benefit { padding-inline: 12px; } .hero-benefit p { max-width: 170px; } .offer-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .faq-section { gap: 35px; } .steps-grid { gap: 22px; } .order-callout { flex-wrap: wrap; } .order-callout > div:nth-child(2) { flex: 1; } .news-grid { grid-template-columns: 1fr; } }
@media (max-width: 600px) {
  .landing-wrap { padding-inline: 20px; } .classic-home { padding-top: 70px; } .hero-inner { padding-top: 47px; padding-bottom: 18px; } .hero-eyebrow { font-size: 10px; gap: 7px; margin-bottom: 23px; }
  h1 { font-size: clamp(30px, 7.5vw, 44px); line-height: 1.4; letter-spacing: -.04em; } .hero-description { font-size: 14px; margin-top: 20px; } .hero-actions { gap: 10px; margin-top: 24px; } .landing-button { padding-inline: 17px; font-size: 13px; }
  .hero-promise { font-size: 10px; align-items: flex-start; line-height: 1.8; } .hero-promise svg { margin-top: 2px; } .hero-benefits { grid-template-columns: 1fr; gap: 18px; margin-top: 33px; max-width: 310px; } .hero-benefit { padding: 0; gap: 15px; } .hero-benefit + .hero-benefit { border: 0; } .hero-benefit p { max-width: none; } .hero-scroll { margin-top: 17px; }
  .landing-section { padding-block: 52px; } .section-heading { display: block; margin-bottom: 24px; } .section-heading > .text-link, .news-links { margin-top: 19px; } .section-heading h2, .faq-intro h2, .closing-section h2 { font-size: 27px; } .section-heading p:not(.section-eyebrow) { font-size: 13px; }
  .offer-grid { grid-template-columns: 1fr; gap: 16px; } .category-links { gap: 7px; } .category-links a { padding: 7px 11px; } .steps-grid { grid-template-columns: 1fr; gap: 30px; margin-top: 32px; } .steps-grid li { padding-left: 85px; min-height: 90px; } .step-number { position: absolute; left: 0; top: 0; font-size: 52px; } .step-icon { top: 36px; left: 34px; width: 22px; } .steps-grid h3 { font-size: 16px; }
  .order-callout { padding: 20px; gap: 14px; } .order-callout .landing-button { width: 100%; } .faq-section { grid-template-columns: 1fr; gap: 28px; } .faq-list summary { font-size: 14px; padding-block: 20px; } .news-section { padding-bottom: 52px; } .campaign-section { padding-bottom: 48px; } .campaign { padding: 24px; } .closing-section { padding-block: 50px; }
}
@media (prefers-reduced-motion: reduce) { .landing-button { transition: none; } .landing-button:hover { transform: none; } }
</style>
