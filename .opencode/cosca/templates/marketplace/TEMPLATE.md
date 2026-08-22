# MARKETPLACE TEMPLATE

## Domain
Online marketplace connecting buyers and sellers. Products, orders, payments, reviews.

## Recommended Stack
- Backend: NestJS or Django
- Frontend: Next.js or Nuxt
- Database: PostgreSQL + Elasticsearch (search)
- Cache: Redis
- Queue: RabbitMQ or Bull
- Storage: S3 for product images
- Payments: Stripe Connect

## Module Structure
```
backend/
├── users/         # Buyers & Sellers
├── products/      # Product listings
├── categories/    # Product categories
├── search/        # Search with Elasticsearch
├── cart/          # Shopping cart
├── orders/        # Order processing
├── payments/      # Payments & payouts (Stripe Connect)
├── reviews/       # Product reviews & ratings
├── messaging/     # Buyer-seller messaging
├── shipping/      # Shipping labels & tracking
└── disputes/      # Dispute resolution
frontend/
├── buyer-app/     # Buyer-facing app
├── seller-app/    # Seller dashboard
└── admin/         # Admin panel
```

## Key Features
- User profiles (buyer + seller roles)
- Product listing with categories and filters
- Full-text search with faceted filtering
- Shopping cart and checkout
- Secure payments with escrow
- Seller payouts
- Order tracking
- Reviews and ratings
- Buyer-seller messaging
- Dispute resolution

## Architecture Notes
- Event-driven order processing
- Elasticsearch for search with PostgreSQL as source of truth
- Stripe Connect for marketplace payments
- Image processing pipeline (resize, optimize, CDN)
- Rate limiting on APIs
