import { useCallback, useEffect, useState } from 'react'
import {
  api,
  Expense,
  SpendingSummary,
  Subscription,
  SaaSSummary,
  UploadResult,
} from './api'

type Tab = 'spending' | 'saas'

export default function App() {
  const [tab, setTab] = useState<Tab>('spending')
  const [expenses, setExpenses] = useState<Expense[]>([])
  const [spendingSummary, setSpendingSummary] = useState<SpendingSummary | null>(null)
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [saasSummary, setSaaSSummary] = useState<SaaSSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [uploadResult, setUploadResult] = useState<UploadResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [e, es, s, ss] = await Promise.all([
        api.spending(),
        api.spendingSummary(),
        api.saas(),
        api.saasSummary(),
      ])
      setExpenses(e)
      setSpendingSummary(es)
      setSubscriptions(s)
      setSaaSSummary(ss)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const onUpload = async (file: File) => {
    setUploading(true)
    setError(null)
    setUploadResult(null)
    try {
      const result = await api.upload(file)
      setUploadResult(result)
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }

  const formatMoney = (amount: number, currency = 'USD') =>
    new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(amount)

  const maxCategory = spendingSummary
    ? Math.max(...Object.values(spendingSummary.by_category), 1)
    : 1

  return (
    <div className="app">
      <header className="header">
        <div>
          <p className="eyebrow">Email Intelligence</p>
          <h1>Receipt & SaaS Insights</h1>
          <p className="subtitle">
            Upload exported inbox data (JSON/CSV). AI extracts spending and discovers subscriptions.
          </p>
        </div>
        <label className={`upload-btn ${uploading ? 'disabled' : ''}`}>
          {uploading ? 'Processing…' : 'Upload emails'}
          <input
            type="file"
            accept=".json,.csv,application/json,text/csv"
            disabled={uploading}
            onChange={(e) => {
              const f = e.target.files?.[0]
              if (f) onUpload(f)
              e.target.value = ''
            }}
          />
        </label>
      </header>

      {uploadResult && (
        <div className="banner success">
          Processed {uploadResult.emails_processed} emails — {uploadResult.expenses_extracted}{' '}
          expenses, {uploadResult.subscriptions_found} subscription signals
        </div>
      )}
      {error && <div className="banner error">{error}</div>}

      <nav className="tabs">
        <button className={tab === 'spending' ? 'active' : ''} onClick={() => setTab('spending')}>
          Spending
        </button>
        <button className={tab === 'saas' ? 'active' : ''} onClick={() => setTab('saas')}>
          SaaS Tools
        </button>
      </nav>

      {loading ? (
        <p className="loading">Loading insights…</p>
      ) : tab === 'spending' ? (
        <SpendingView
          expenses={expenses}
          summary={spendingSummary}
          formatMoney={formatMoney}
          maxCategory={maxCategory}
        />
      ) : (
        <SaaSView subscriptions={subscriptions} summary={saasSummary} formatMoney={formatMoney} />
      )}
    </div>
  )
}

function SpendingView({
  expenses,
  summary,
  formatMoney,
  maxCategory,
}: {
  expenses: Expense[]
  summary: SpendingSummary | null
  formatMoney: (n: number, c?: string) => string
  maxCategory: number
}) {
  return (
    <div className="grid">
      <section className="card stats-row">
        <Stat label="Total spent" value={summary ? formatMoney(summary.total_amount) : '—'} />
        <Stat label="Transactions" value={String(summary?.transaction_count ?? 0)} />
        <Stat
          label="Categories"
          value={String(Object.keys(summary?.by_category ?? {}).length)}
        />
      </section>

      <section className="card">
        <h2>By category</h2>
        <div className="bars">
          {summary &&
            Object.entries(summary.by_category)
              .sort((a, b) => b[1] - a[1])
              .map(([cat, amt]) => (
                <div key={cat} className="bar-row">
                  <span className="bar-label">{cat}</span>
                  <div className="bar-track">
                    <div className="bar-fill spending" style={{ width: `${(amt / maxCategory) * 100}%` }} />
                  </div>
                  <span className="bar-value">{formatMoney(amt)}</span>
                </div>
              ))}
          {summary && Object.keys(summary.by_category).length === 0 && (
            <p className="empty">Upload sample emails to see categories.</p>
          )}
        </div>
      </section>

      <section className="card wide">
        <h2>Transactions</h2>
        <table>
          <thead>
            <tr>
              <th>Date</th>
              <th>Merchant</th>
              <th>Category</th>
              <th>Amount</th>
            </tr>
          </thead>
          <tbody>
            {expenses.map((ex) => (
              <tr key={ex.id}>
                <td>{ex.expense_date ? ex.expense_date.slice(0, 10) : '—'}</td>
                <td>{ex.merchant}</td>
                <td>
                  <span className="pill">{ex.category}</span>
                </td>
                <td className="mono">{formatMoney(ex.amount, ex.currency)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {expenses.length === 0 && <p className="empty">No expenses yet.</p>}
      </section>
    </div>
  )
}

function SaaSView({
  subscriptions,
  summary,
  formatMoney,
}: {
  subscriptions: Subscription[]
  summary: SaaSSummary | null
  formatMoney: (n: number, c?: string) => string
}) {
  return (
    <div className="grid">
      <section className="card stats-row">
        <Stat label="Active tools" value={String(summary?.active_count ?? 0)} />
        <Stat
          label="Est. monthly"
          value={summary ? formatMoney(summary.estimated_monthly, summary.currency) : '—'}
        />
        <Stat label="Services tracked" value={String(summary?.services.length ?? 0)} />
      </section>

      <section className="card">
        <h2>Billing cycles</h2>
        <div className="chips">
          {summary &&
            Object.entries(summary.by_billing_cycle).map(([cycle, count]) => (
              <span key={cycle} className="chip">
                {cycle}: {count}
              </span>
            ))}
        </div>
      </section>

      <section className="card wide">
        <h2>Subscriptions</h2>
        <div className="saas-grid">
          {subscriptions.map((sub) => (
            <article key={sub.id} className="saas-card">
              <h3>{sub.service_name}</h3>
              <p className="meta">{sub.plan || 'Plan unknown'}</p>
              <p className="amount">
                {sub.amount != null ? formatMoney(sub.amount, sub.currency) : '—'}
                {sub.billing_cycle && <span className="cycle"> / {sub.billing_cycle}</span>}
              </p>
              <div className="saas-footer">
                <span className={`status ${sub.status}`}>{sub.status}</span>
                <span className="signal">{sub.signal_type}</span>
              </div>
            </article>
          ))}
        </div>
        {subscriptions.length === 0 && <p className="empty">No SaaS tools detected yet.</p>}
      </section>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
    </div>
  )
}
