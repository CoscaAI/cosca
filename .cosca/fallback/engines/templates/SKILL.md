> **Version**: 1.0.0 | **Status**: active | **Owner**: Template Engine | **Last Updated**: 2026-07-10

# TEMPLATE ENGINE

## PURPOSE
The Template Engine provides project scaffolding templates for different application types. It generates complete project structures with best practices and Cosca integration.

## TEMPLATE TYPES

### 1. ERP (Enterprise Resource Planning)
```
erp-project/
├── apps/
│   ├── api/               # Backend API
│   ├── web/               # Frontend dashboard
│   └── mobile/            # Mobile app (optional)
├── packages/
│   ├── shared/            # Shared types and utilities
│   ├── ui/                # Design system
│   └── config/            # Shared configuration
├── .cosca/
│   └── (Cosca integration files)
├── docker-compose.yml
└── README.md
```

### 2. CRM (Customer Relationship Management)
```
crm-project/
├── backend/
│   ├── src/
│   │   ├── contacts/
│   │   ├── deals/
│   │   ├── activities/
│   │   └── reports/
│   └── tests/
├── frontend/
│   ├── src/
│   │   ├── pages/
│   │   ├── components/
│   │   └── hooks/
│   └── tests/
├── .cosca/
└── README.md
```

### 3. SaaS (Software as a Service)
```
saas-project/
├── apps/
│   ├── web/               # Main SaaS app
│   ├── admin/             # Admin dashboard
│   ├── api/               # REST/GraphQL API
│   └── landing/           # Marketing site
├── packages/
│   ├── shared/
│   ├── billing/           # Subscription management
│   ├── auth/              # Authentication
│   └── email/             # Email templates
├── .cosca/
└── README.md
```

### 4. Marketplace
```
marketplace-project/
├── backend/
│   ├── users/
│   ├── products/
│   ├── orders/
│   ├── payments/
│   ├── search/
│   └── notifications/
├── frontend/
│   ├── buyer-app/
│   └── seller-app/
├── admin/
├── .cosca/
└── README.md
```

### 5. Mobile App
```
mobile-project/
├── app/                    # Expo/React Native app
├── api/                    # Backend API
├── shared/
├── admin-web/              # Admin panel
├── .cosca/
└── README.md
```

### 6. API (Pure API/Backend)
```
api-project/
├── src/
│   ├── modules/
│   ├── common/
│   ├── config/
│   └── main.ts
├── tests/
│   ├── unit/
│   ├── integration/
│   └── e2e/
├── docs/
├── .cosca/
└── README.md
```

### 7. Microservices
```
microservices-project/
├── services/
│   ├── gateway/            # API Gateway
│   ├── auth/               # Auth service
│   ├── users/              # User service
│   ├── products/           # Product service
│   ├── orders/             # Order service
│   └── notifications/      # Notification service
├── packages/
│   ├── shared/
│   └── contracts/
├── infrastructure/
│   ├── docker/
│   ├── k8s/
│   └── terraform/
├── .cosca/
└── README.md
```

### 8. Landing Page
```
landing-project/
├── src/
│   ├── components/
│   ├── sections/
│   ├── pages/
│   └── styles/
├── public/
│   └── images/
├── .cosca/
└── README.md
```

### 9. Admin Panel
```
admin-project/
├── src/
│   ├── pages/
│   ├── components/
│   ├── layouts/
│   ├── hooks/
│   └── services/
├── .cosca/
└── README.md
```

## TEMPLATE INITIALIZATION PROCESS

1. Identify project type from wizard or user request
2. Load appropriate template
3. Customize with project name and configuration
4. Generate directory structure
5. Initialize Cosca integration files
6. Create initial README with setup instructions
7. Set up code quality tools (linter, formatter, type checker)
8. Create initial .gitignore
9. Set up CI/CD template
10. Initialize git repository

## Cosca INTEGRATION FILES (per project)
```
.cosca/
├── config.yml             # Project Cosca configuration
├── state.yml              # Current project state
├── memory/                # Project memory (symlink or copy)
└── workflows/             # Active workflows
```

## BEST PRACTICES PER TEMPLATE
Each template includes:
- Recommended tech stack
- Architecture pattern
- Testing strategy
- Security baseline
- Performance baseline
- Deployment strategy
- Scaling strategy

## RELATED
- [Wizard Engine](../wizard/SKILL.md) — Feature intake may select templates for scaffolding

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-10 | Cosca Refactor | Added metadata, HISTORY, and cross-references |
