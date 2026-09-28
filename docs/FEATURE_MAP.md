# Ruomu Card Platform: capability map

Verified from the local checkout on 2026-09-29. This is a scoped navigation aid, not a complete product audit or a replacement for [existing guidance](../README.md). Recheck implementation when using it; extend entries only as tasks confirm the relevant behavior. Do not treat roadmap ideas as implemented features.

| Capability | Entry / UI | API / orchestration | Implementation / data | Review boundary |
|---|---|---|---|---|
| User storefront and admin console | [apps/user/src/router](../apps/user/src/router) | [apps/admin/src/router](../apps/admin/src/router) | [apps/api/internal/router/router.go](../apps/api/internal/router/router.go) | Keep api, user and admin independently built applications; compare both user and admin flows. |
| Orders, payment and fulfillment | [apps/api/internal/service/payment_service_capture.go](../apps/api/internal/service/payment_service_capture.go) | [apps/api/internal/service/fulfillment_service.go](../apps/api/internal/service/fulfillment_service.go) | [apps/api/internal/repository](../apps/api/internal/repository) | Payment confirmation, inventory and delivery are separate stages; inspect retry/idempotency behavior before modifying them. |
| Payment callbacks and providers | [apps/api/internal/router/router.go](../apps/api/internal/router/router.go) | [apps/api/internal/payment](../apps/api/internal/payment) | [apps/api/internal/worker](../apps/api/internal/worker) | GET/POST /api/v1/payments/callback and provider webhooks need their existing verification and access boundaries. |
| Build and deployment | [scripts/build.sh](../scripts/build.sh) | [scripts/deploy.sh](../scripts/deploy.sh) | [deploy](../deploy) | Preserve three-app builds and TLS/callback checks; never record real keys or certificates in notes. |

Retain separate Go API and user/admin frontends. Existing scripts pin pnpm for application work; Bun is only the note-check runtime.

Before adding a feature, inspect adjacent flows and the current source of truth; shared filters, data definitions and access rules must not diverge across entry points.
