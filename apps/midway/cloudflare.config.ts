import { defineConfig } from '@cloudflare/config'

/**
 * Cloudflare deployment configuration (CLI `cf` v1).
 *
 * The `cf` v1 beta reads `cloudflare.config.ts` — not `wrangler.toml`. Custom
 * domains go in `worker.domains`, which the CLI translates into a route with
 * `custom_domain: true`, creating the DNS record and the TLS certificate.
 *
 * Deploy with: `cf deploy dist`
 */
export default defineConfig({
  worker: {
    name: 'midway',
    compatibilityDate: '2026-11-13',
    previewUrls: false,
    assets: {
      notFoundHandling: 'single-page-application',
    },
    domains: [
      {
        hostname: 'midway.amarrete.com',
        production: true,
        previews: false,
      },
    ],
  },
})
