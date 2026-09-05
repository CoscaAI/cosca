# SAAS TEMPLATE

## Domain
Software as a Service. Multi-tenant, subscription-based applications.

## Recommended Stack
- Backend: NestJS or Next.js API routes
- Frontend: Next.js (SSR + CSR)
- Database: PostgreSQL + Redis
- Payments: Stripe or Paddle
- Email: Resend, Postmark, or SendGrid
- File Storage: S3 or Cloudflare R2
- Analytics: PostHog or custom

## Module Structure
```
apps/
├── web/           # Main SaaS App
├── admin/         # Admin Dashboard
├── api/           # API (or use Next.js API routes)
└── landing/       # Marketing Site
packages/
├── shared/        # Types and utilities
├── ui/            # Design system
├── billing/       # Subscription & billing
├── auth/          # Authentication (NextAuth, Clerk)
├── email/         # Transactional email templates
├── storage/       # File upload/download
└── analytics/     # Event tracking
```

## Key Features
- User registration and authentication
- Multi-tenant data isolation
- Subscription plans (Stripe)
- Team/organization management
- Role-based access control
- Billing and invoicing
- Usage tracking and limits
- Onboarding flow
- Feature flags per plan

## Architecture Notes
- Row-level security for multi-tenancy
- Plan-based feature gating
- Usage metering and quotas
- Webhook handling for payment events
- Background jobs for async tasks
