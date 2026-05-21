# Email Insights

Analyze exported inbox data (JSON/CSV) to extract **spending** from receipts and discover **SaaS subscriptions** from welcome emails, invoices, and renewal notices.

## Quick start

```bash
cp .env.example .env
docker compose up --build
```

Open **http://localhost:5173**, upload `sample-data/emails.json`, and explore the Spending and SaaS tabs.

### With OpenAI (optional)

```bash
# .env
OPENAI_API_KEY=sk-...
LLM_MOCK_MODE=false
```

Restart the API: `docker compose up api --build`

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/emails/upload` | Multipart form field `file` (JSON or CSV) |
| GET | `/api/spending` | List extracted expenses |
| GET | `/api/spending/summary` | Totals, by category, top merchants |
| GET | `/api/saas` | Detected subscriptions |
| GET | `/api/saas/summary` | Active count, estimated monthly spend |
| GET | `/health` | Health check |

### Upload format

JSON array or `{ "emails": [...] }`:

```json
{
  "id": "optional-external-id",
  "subject": "Your receipt from Amazon",
  "from": "orders@amazon.com",
  "to": "you@example.com",
  "date": "2024-03-15T14:22:00Z",
  "body": "Order total: $34.99"
}
```

CSV columns: `id`, `subject`, `from`/`sender`, `to`/`recipient`, `body`, `date`/`received_at`.

## Local development (without Docker)

**Database**

```bash
docker run -d --name insights-pg -e POSTGRES_USER=insights -e POSTGRES_PASSWORD=insights -e POSTGRES_DB=insights -p 5432:5432 postgres:16-alpine
```

**Backend**

```bash
cd backend
export DATABASE_URL=postgres://insights:insights@localhost:5432/insights?sslmode=disable
export LLM_MOCK_MODE=true
go run ./cmd/server
```

**Frontend**

```bash
cd frontend
npm install
npm run dev
```

Vite proxies `/api` to `localhost:8080`.

## Architecture

```
┌─────────────┐     REST      ┌──────────────────────────────────────┐
│ React UI    │ ────────────► │ Go API (chi)                         │
│ upload/view │               │  parser → analyzer → service → store │
└─────────────┘               └──────────────┬───────────────────────┘
                                             │
                    ┌────────────────────────┼────────────────────────┐
                    ▼                        ▼                        ▼
              PostgreSQL              OpenAI (optional)         Heuristic mock
              emails, expenses,       structured JSON           when no API key
              subscriptions
```

**Layers**

- `internal/parser` — JSON/CSV ingestion
- `internal/analyzer` — `Analyzer` interface; OpenAI implementation with mock fallback
- `internal/service` — orchestrates classify → persist
- `internal/store` — PostgreSQL via pgx; subscription upsert by service name
- `internal/api` — HTTP handlers

**Design decisions**

1. **File upload first** — OAuth/IMAP is out of scope; export upload matches the brief and keeps auth simple.
2. **Analyzer interface** — swap OpenAI, Anthropic, or local models without touching handlers.
3. **Mock mode by default** — runnable without API keys; heuristics handle the sample dataset.
4. **Subscription upsert** — multiple emails (welcome + invoice + renewal) merge into one row per service.
5. **Truncated bodies** — LLM prompts cap body length to control cost/latency.

## What I'd improve next

- [ ] **OAuth inbox sync** (Gmail/Microsoft Graph) with incremental fetch and deduplication
- [ ] **Background job queue** (e.g. River/temporal) for large uploads instead of synchronous processing
- [ ] **User/auth multi-tenancy** — all tables scoped by `user_id`
- [ ] **Deduplication** — hash `(sender, subject, date, amount)` to avoid double-counting
- [ ] **Anthropic provider** and eval fixtures for extraction quality
- [ ] **Pagination & filters** on list endpoints; date-range query params
- [ ] **Tests** — parser unit tests, analyzer golden files, API integration tests with testcontainers
- [ ] **Charts** — time-series spending, renewal calendar for SaaS

## Time spent

~3 hours (scaffolding, backend, frontend, Docker, sample data, documentation).

## License

MIT (take-home submission).
