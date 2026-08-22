# ADMIN PANEL TEMPLATE

## Domain
Admin dashboard for managing application data, users, and settings.

## Recommended Stack
- Frontend: React + Ant Design or MUI
- Backend: NestJS or FastAPI (often shared with main app)
- Auth: RBAC with fine-grained permissions
- Table: TanStack Table or AG Grid
- Charts: Recharts, Tremor, or Chart.js
- Forms: React Hook Form + Zod

## Module Structure
```
src/
├── pages/
│   ├── dashboard/     # Overview with KPIs and charts
│   ├── users/         # User management (list, edit, create)
│   ├── roles/         # Role and permission management
│   ├── settings/      # Application settings
│   └── [domain]/      # Domain-specific CRUD pages
├── components/
│   ├── layout/        # Admin layout (sidebar, header, breadcrumbs)
│   ├── tables/        # Data table with sorting, filtering, pagination
│   ├── forms/         # Form components
│   ├── charts/        # Chart components
│   └── shared/        # Shared components
├── hooks/
│   ├── useAuth.ts
│   ├── usePermissions.ts
│   └── useCrud.ts
├── services/          # API client functions
├── types/             # TypeScript types
└── utils/             # Utilities
```

## Key Features
- Responsive sidebar navigation
- User authentication with RBAC
- CRUD interfaces for all entities
- Data tables with sorting, filtering, pagination
- Export to CSV/Excel/PDF
- Bulk actions (delete, update status)
- Activity log / audit trail
- Dashboard with KPIs
- Search across entities
- Notifications (in-app and email)
- Dark/light mode
- Mobile responsive

## Page Templates
1. Dashboard — Stats cards, charts, recent activity
2. List Page — Search, filters, table, pagination, bulk actions
3. Create/Edit Page — Form with validation, file upload
4. Detail Page — Read-only view with actions
5. Settings Page — Form-based configuration
