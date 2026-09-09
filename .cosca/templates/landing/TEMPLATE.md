# LANDING PAGE TEMPLATE

## Domain
Marketing landing page or simple website. Fast, SEO-optimized, mobile-first.

## Recommended Stack
- Framework: Next.js (SSG) or Astro
- Styling: Tailwind CSS
- Components: shadcn/ui or custom
- Analytics: PostHog, Plausible, or Google Analytics
- Forms: React Hook Form + Zod
- Email: Resend or SendGrid
- Hosting: Vercel or Cloudflare Pages
- CMS (optional): Strapi or Contentful

## Module Structure
```
src/
├── app/               # Next.js App Router
│   ├── page.tsx       # Home page
│   ├── about/
│   ├── pricing/
│   ├── blog/
│   ├── contact/
│   └── layout.tsx     # Root layout
├── components/
│   ├── sections/      # Page sections (Hero, Features, Pricing, FAQ, CTA, Footer)
│   ├── ui/            # Reusable UI components
│   └── layout/        # Header, Footer, Navigation
├── lib/
│   ├── utils.ts
│   ├── constants.ts
│   └── fonts.ts
├── styles/
│   └── globals.css
└── content/           # MDX blog posts (optional)
public/
├── images/
├── fonts/
└── favicon.ico
```

## Key Features
- Responsive design (mobile-first)
- SEO optimized (meta tags, sitemap, robots.txt)
- Fast loading (static generation, image optimization)
- Contact form with validation
- Email capture (newsletter signup)
- Analytics integration
- Cookie consent banner
- Accessibility (WCAG AA)
- Dark/light mode (optional)
- Multi-language (optional, via next-intl)

## Section Templates
1. Hero — Headline, subheadline, CTA, hero image
2. Features — Grid of feature cards with icons
3. Testimonials — Customer quotes carousel
4. Pricing — Pricing cards comparison
5. FAQ — Accordion of frequently asked questions
6. CTA — Call to action banner
7. Footer — Links, social media, copyright
