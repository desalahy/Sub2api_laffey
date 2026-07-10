import type { ComposerTranslation } from 'vue-i18n'

const DEFAULT_SITE_SUBTITLES = new Set([
  '订阅转 API 中转平台',
  '订阅转 API 转换平台',
  'Subscription to API Conversion Platform',
  'AI API Gateway Platform'
])

export function resolveSiteSubtitle(subtitle: string | null | undefined, t: ComposerTranslation): string {
  const normalized = subtitle?.trim()

  if (!normalized || DEFAULT_SITE_SUBTITLES.has(normalized)) {
    return t('home.siteSubtitle')
  }

  return normalized
}
