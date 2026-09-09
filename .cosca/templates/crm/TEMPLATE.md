# CRM TEMPLATE

## Domain
Customer Relationship Management. Contacts, deals, pipeline, activities.

## Recommended Stack
- Backend: FastAPI or Express + TypeScript
- Frontend: React + Tailwind or Vue + Vuetify
- Database: PostgreSQL + Redis
- Real-time: WebSockets for notifications
- Email: SendGrid, SES, or Resend

## Module Structure
```
backend/
├── contacts/      # Contacts & Companies
├── deals/         # Deals & Pipeline
├── activities/    # Calls, Meetings, Tasks, Notes
├── email/         # Email integration & tracking
├── reports/       # Sales reports & forecasts
└── integrations/  # Calendar, email, Slack integrations
frontend/
├── pages/
│   ├── dashboard/
│   ├── contacts/
│   ├── deals/
│   ├── activities/
│   └── settings/
├── components/
│   ├── pipeline/   # Kanban board
│   ├── forms/      # Quick-add forms
│   └── shared/     # Common components
└── hooks/          # Custom React hooks
```

## Key Features
- Contact management with custom fields
- Visual sales pipeline (Kanban)
- Activity tracking (calls, meetings, tasks)
- Email integration (send/receive/track)
- Sales forecasting
- Custom reports and dashboards
- Import/export contacts
- Team collaboration (comments, mentions)
