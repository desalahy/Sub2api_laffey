<template>
  <div class="relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <!-- Background -->
    <div
      class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
    ></div>

    <!-- Decorative Elements -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <!-- Gradient Orbs -->
      <div
        class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"
      ></div>
      <div
        class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"
      ></div>

      <!-- Grid Pattern -->
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="mb-8 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div
            class="mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl shadow-lg shadow-primary-500/30"
          >
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="text-gradient mb-2 text-3xl font-bold">
            {{ siteName }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="auth-card-wrap">
        <img
          class="auth-laffey-sticker"
          src="/assets/laffey/chibi/laffey-snack-chibi.png"
          alt=""
          aria-hidden="true"
        />
        <div class="auth-form-card card-glass relative z-10 rounded-2xl p-8 shadow-glass">
          <slot />
        </div>
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { DEFAULT_SITE_NAME } from '@/constants/branding'
import { sanitizeUrl } from '@/utils/url'
import { useI18n } from 'vue-i18n'
import { resolveSiteSubtitle } from '@/utils/siteSubtitle'

const appStore = useAppStore()
const { t } = useI18n()

const siteName = computed(() => appStore.siteName || DEFAULT_SITE_NAME)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => resolveSiteSubtitle(appStore.cachedPublicSettings?.site_subtitle, t))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.text-gradient {
  @apply bg-gradient-to-r from-primary-600 to-primary-500 bg-clip-text text-transparent;
}

.auth-card-wrap {
  position: relative;
}

.auth-card-wrap::before {
  content: '';
  position: absolute;
  right: -18px;
  top: -18px;
  width: 118px;
  height: 118px;
  border-radius: 9999px;
  background: radial-gradient(circle, rgba(20, 184, 166, 0.18), transparent 68%);
  filter: blur(18px);
  opacity: 0.65;
}

.auth-card-wrap::after {
  content: '';
  pointer-events: none;
  position: absolute;
  inset: 1px;
  z-index: 11;
  border-radius: 1rem;
  background:
    radial-gradient(circle at 90% 6%, rgba(244, 114, 182, 0.13), transparent 24%),
    radial-gradient(circle at 10% 96%, rgba(20, 184, 166, 0.12), transparent 26%);
  opacity: 0.72;
}

.auth-form-card {
  overflow: hidden;
}

.auth-laffey-sticker {
  pointer-events: none;
  position: absolute;
  right: -46px;
  top: -58px;
  z-index: 20;
  width: 112px;
  height: auto;
  object-fit: contain;
  opacity: 0.94;
  transform: rotate(7deg);
  filter: drop-shadow(0 14px 22px rgba(15, 23, 42, 0.16));
  transition:
    opacity 0.2s ease,
    transform 0.2s ease;
}

.auth-card-wrap:hover .auth-laffey-sticker {
  opacity: 1;
  transform: rotate(4deg) translateY(-3px);
}

:deep(.dark) .auth-card-wrap::before {
  background: radial-gradient(circle, rgba(20, 184, 166, 0.14), transparent 68%);
  opacity: 0.5;
}

:deep(.dark) .auth-card-wrap::after {
  background:
    radial-gradient(circle at 90% 6%, rgba(244, 114, 182, 0.09), transparent 24%),
    radial-gradient(circle at 10% 96%, rgba(20, 184, 166, 0.1), transparent 26%);
  opacity: 0.56;
}

:deep(.dark) .auth-laffey-sticker {
  opacity: 0.9;
  filter: drop-shadow(0 14px 26px rgba(20, 184, 166, 0.18));
}

@media (max-width: 520px) {
  .auth-laffey-sticker {
    display: none;
  }

  .auth-card-wrap::before {
    display: none;
  }
}
</style>
