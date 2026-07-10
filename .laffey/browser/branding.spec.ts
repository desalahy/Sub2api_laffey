import { expect, test } from '@playwright/test'

const baseURL = process.env.LAFFEY_PREVIEW_URL || 'http://127.0.0.1:3000'

for (const viewport of [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'mobile', width: 390, height: 844 },
]) {
  for (const route of ['/home', '/login', '/register']) {
    test(`${viewport.name} ${route} renders Laffey without browser errors`, async ({ page }, testInfo) => {
      const errors: string[] = []
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text())
      })
      page.on('pageerror', (error) => errors.push(error.message))
      await page.setViewportSize({ width: viewport.width, height: viewport.height })

      await page.goto(`${baseURL}${route}`, { waitUntil: 'networkidle' })

      await expect(page.locator('body')).toBeVisible()
      await expect(page.getByText('Laffey API', { exact: false }).first()).toBeVisible()
      await page.screenshot({
        path: testInfo.outputPath(`${viewport.name}-${route.slice(1)}.png`),
        fullPage: true,
      })
      expect(errors).toEqual([])
    })
  }
}

