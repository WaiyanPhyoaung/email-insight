import { useCallback, useEffect, useState } from 'react'
import {
  api,
  Expense,
  SpendingSummary,
  Subscription,
  SaaSSummary,
  UploadResult,
  Email,
} from './api'

type Tab = 'spending' | 'saas'
type SortField = 'date' | 'amount' | 'merchant' | 'confidence' | 'service_name'
type SortOrder = 'asc' | 'desc'

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
  const [errorTitle, setErrorTitle] = useState('Request issue')
  const [dragActive, setDragActive] = useState(false)

  // Filtering & Sorting State
  const [spendingSearch, setSpendingSearch] = useState('')
  const [spendingCategory, setSpendingCategory] = useState('')
  const [saasSearch, setSaasSearch] = useState('')
  const [saasStatus, setSaasStatus] = useState('')
  const [sortField, setSortField] = useState<SortField>('date')
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc')

  // Selected Email for Drawer
  const [selectedEmailId, setSelectedEmailId] = useState<string | null>(null)

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
      setErrorTitle('Dashboard unavailable')
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
      setErrorTitle('Upload issue')
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setUploading(false)
    }
  }

  // Drag and Drop Handlers
  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true)
    } else if (e.type === 'dragleave') {
      setDragActive(false)
    }
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragActive(false)
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      onUpload(e.dataTransfer.files[0])
    }
  }

  const formatMoney = (amount: number, currency = 'USD') =>
    new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(amount)

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortOrder('desc')
    }
  }

  return (
    <div className="app-container">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="brand-logo" aria-hidden="true">EI</div>
          <div>
            <h2>EmailInsight</h2>
            <p className="eyebrow">Inbox finance</p>
          </div>
        </div>

        <nav className="nav-menu">
          <button
            className={`nav-item ${tab === 'spending' ? 'active' : ''}`}
            onClick={() => {
              setTab('spending')
              setSortField('date')
              setSortOrder('desc')
            }}
          >
            <span className="nav-icon" aria-hidden="true">S</span> Spending
          </button>
          <button
            className={`nav-item ${tab === 'saas' ? 'active' : ''}`}
            onClick={() => {
              setTab('saas')
              setSortField('merchant')
              setSortOrder('asc')
            }}
          >
            <span className="nav-icon" aria-hidden="true">R</span> Subscriptions
          </button>
        </nav>

        <div className="sidebar-footer">
          <p>Local workspace</p>
          <div className="badge">Private</div>
        </div>
      </aside>

      <main className="main-content">
        <header className="header">
          <div>
            <h1>{tab === 'spending' ? 'Spending Analysis' : 'SaaS Discovery'}</h1>
            <p className="subtitle">
              Review uploaded receipts, invoices, renewals, and trial emails in one focused workspace.
            </p>
          </div>

          <div className="header-actions">
            <button className="refresh-btn" onClick={refresh} title="Refresh data" disabled={loading || uploading}>
              Refresh
            </button>
          </div>
        </header>

        <div
          className={`dropzone ${dragActive ? 'drag-active' : ''} ${uploading ? 'processing' : ''}`}
          onDragEnter={handleDrag}
          onDragOver={handleDrag}
          onDragLeave={handleDrag}
          onDrop={handleDrop}
        >
          <div className="dropzone-inner">
            <span className="drop-icon" aria-hidden="true">{uploading ? '...' : 'CSV'}</span>
            <div>
              <h3>{uploading ? 'Processing upload' : 'Upload inbox export'}</h3>
              <p>JSON or CSV with subject, sender, body, and optional date fields.</p>
            </div>
            <label className="file-input-label">
              Choose file
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
          </div>
        </div>

        {uploadResult && (
          <div className="banner success">
            <div>
              <strong>Upload processed</strong>
              <p>
                Parsed {uploadResult.emails_processed} new emails, extracted{' '}
                {uploadResult.expenses_extracted} expenses and found{' '}
                {uploadResult.subscriptions_found} subscriptions.
              </p>
            </div>
            <button className="banner-close" onClick={() => setUploadResult(null)}>
              &times;
            </button>
          </div>
        )}

        {error && (
          <div className="banner error">
            <div>
              <strong>{errorTitle}</strong>
              <p>{error}</p>
            </div>
            <button className="banner-close" onClick={() => setError(null)}>
              &times;
            </button>
          </div>
        )}

        {loading ? (
          <div className="loading-state">
            <div className="spinner"></div>
            <p>Loading dashboard data...</p>
          </div>
        ) : tab === 'spending' ? (
          <SpendingView
            expenses={expenses}
            summary={spendingSummary}
            formatMoney={formatMoney}
            onSelectEmail={setSelectedEmailId}
            spendingSearch={spendingSearch}
            setSpendingSearch={setSpendingSearch}
            spendingCategory={spendingCategory}
            setSpendingCategory={setSpendingCategory}
            sortField={sortField}
            sortOrder={sortOrder}
            onSort={handleSort}
          />
        ) : (
          <SaaSView
            subscriptions={subscriptions}
            summary={saasSummary}
            formatMoney={formatMoney}
            onSelectEmail={setSelectedEmailId}
            saasSearch={saasSearch}
            setSaasSearch={setSaasSearch}
            saasStatus={saasStatus}
            setSaasStatus={setSaasStatus}
            sortField={sortField}
            sortOrder={sortOrder}
            onSort={handleSort}
          />
        )}
      </main>

      {/* Slide-out Drawer */}
      {selectedEmailId && (
        <EmailDrawer
          emailId={selectedEmailId}
          onClose={() => setSelectedEmailId(null)}
        />
      )}
    </div>
  )
}

function DonutChart({
  data,
  formatMoney,
}: {
  data: Record<string, number>
  formatMoney: (n: number) => string
}) {
  const total = Object.values(data).reduce((a, b) => a + b, 0)
  if (total <= 0) return <p className="empty-chart">No category data to display</p>

  let accumulated = 0
  const slices = Object.entries(data)
    .sort((a, b) => b[1] - a[1])
    .map(([name, val]) => {
      const pct = (val / total) * 100
      const offset = 100 - accumulated
      accumulated += pct
      return { name, val, pct, offset }
    })

  return (
    <div className="donut-chart-wrapper">
      <div className="donut-svg-container">
        <svg viewBox="0 0 42 42" className="donut-svg">
          <circle
            cx="21"
            cy="21"
            r="15.91549430918954"
            fill="transparent"
            stroke="var(--border)"
            strokeWidth="3.5"
          />
          {slices.map((slice, i) => (
            <circle
              key={slice.name}
              cx="21"
              cy="21"
              r="15.91549430918954"
              fill="transparent"
              stroke={`var(--chart-${i % 6})`}
              strokeWidth="4"
              strokeDasharray={`${slice.pct} ${100 - slice.pct}`}
              strokeDashoffset={slice.offset}
              transform="rotate(-90 21 21)"
              className="donut-slice"
            />
          ))}
          <g className="donut-center-text">
            <text x="21" y="20" className="donut-total">
              {formatMoney(total)}
            </text>
            <text x="21" y="25" className="donut-label">
              Total USD
            </text>
          </g>
        </svg>
      </div>

      <div className="donut-legend">
        {slices.map((slice, i) => (
          <div key={slice.name} className="legend-item">
            <span className="legend-badge" style={{ backgroundColor: `var(--chart-${i % 6})` }} />
            <span className="legend-name">{slice.name}</span>
            <span className="legend-pct">{slice.pct.toFixed(0)}%</span>
            <span className="legend-value">{formatMoney(slice.val)}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function SpendingView({
  expenses,
  summary,
  formatMoney,
  onSelectEmail,
  spendingSearch,
  setSpendingSearch,
  spendingCategory,
  setSpendingCategory,
  sortField,
  sortOrder,
  onSort,
}: {
  expenses: Expense[]
  summary: SpendingSummary | null
  formatMoney: (n: number, c?: string) => string
  onSelectEmail: (id: string) => void
  spendingSearch: string
  setSpendingSearch: (s: string) => void
  spendingCategory: string
  setSpendingCategory: (s: string) => void
  sortField: SortField
  sortOrder: SortOrder
  onSort: (field: SortField) => void
}) {
  const categories = Array.from(new Set(expenses.map((e) => e.category))).filter(Boolean)

  const filtered = expenses
    .filter((ex) => {
      const matchSearch = ex.merchant.toLowerCase().includes(spendingSearch.toLowerCase())
      const matchCat = spendingCategory === '' || ex.category === spendingCategory
      return matchSearch && matchCat
    })
    .sort((a, b) => {
      let valA: any = a[sortField === 'merchant' ? 'merchant' : sortField === 'amount' ? 'amount' : 'expense_date']
      let valB: any = b[sortField === 'merchant' ? 'merchant' : sortField === 'amount' ? 'amount' : 'expense_date']

      if (sortField === 'date') {
        valA = a.expense_date ? new Date(a.expense_date).getTime() : 0
        valB = b.expense_date ? new Date(b.expense_date).getTime() : 0
      }

      if (valA < valB) return sortOrder === 'asc' ? -1 : 1
      if (valA > valB) return sortOrder === 'asc' ? 1 : -1
      return 0
    })

  return (
    <div className="dashboard-grid">
      <section className="stats-cards-row">
        <div className="stat-card">
          <span className="stat-icon" aria-hidden="true">$</span>
          <div>
            <span className="stat-label">Total Spend</span>
            <span className="stat-value">
              {summary ? formatMoney(summary.total_amount) : '$0.00'}
            </span>
          </div>
        </div>

        <div className="stat-card">
          <span className="stat-icon" aria-hidden="true">#</span>
          <div>
            <span className="stat-label">Transactions</span>
            <span className="stat-value">{summary?.transaction_count ?? 0}</span>
          </div>
        </div>

        <div className="stat-card">
          <span className="stat-icon" aria-hidden="true">C</span>
          <div>
            <span className="stat-label">Categories</span>
            <span className="stat-value">
              {Object.keys(summary?.by_category ?? {}).length}
            </span>
          </div>
        </div>
      </section>

      <section className="card card-chart">
        <div className="card-header">
          <h2>Category Breakdown</h2>
          <span className="subtitle">Expenses normalized to USD</span>
        </div>
        {summary ? <DonutChart data={summary.by_category} formatMoney={formatMoney} /> : <p className="empty-chart">Upload emails to see category data.</p>}
      </section>

      <section className="card wide">
        <div className="card-header list-header">
          <div>
            <h2>Transaction Records</h2>
            <span className="subtitle">Showing {filtered.length} of {expenses.length} results</span>
          </div>

          <div className="filter-bar">
            <input
              type="text"
              placeholder="Search merchants"
              className="search-input"
              value={spendingSearch}
              onChange={(e) => setSpendingSearch(e.target.value)}
            />
            <select
              className="filter-select"
              value={spendingCategory}
              onChange={(e) => setSpendingCategory(e.target.value)}
            >
              <option value="">All Categories</option>
              {categories.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="table-responsive">
          <table>
            <thead>
              <tr>
                <th onClick={() => onSort('date')} className="sortable">
                  Date {sortField === 'date' ? (sortOrder === 'asc' ? '▲' : '▼') : ''}
                </th>
                <th onClick={() => onSort('merchant')} className="sortable">
                  Merchant {sortField === 'merchant' ? (sortOrder === 'asc' ? '▲' : '▼') : ''}
                </th>
                <th>Category</th>
                <th onClick={() => onSort('amount')} className="sortable text-right">
                  Amount {sortField === 'amount' ? (sortOrder === 'asc' ? '▲' : '▼') : ''}
                </th>
                <th>Confidence</th>
                <th>Source</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((ex) => (
                <tr key={ex.id}>
                  <td>{ex.expense_date ? ex.expense_date.slice(0, 10) : '—'}</td>
                  <td className="merchant-name">{ex.merchant}</td>
                  <td>
                    <span className="pill-category">{ex.category}</span>
                  </td>
                  <td className="amount-val mono text-right">
                    {formatMoney(ex.amount, ex.currency)}
                  </td>
                  <td>
                    <div className="confidence-indicator" title={`${(ex.confidence * 100).toFixed(0)}% Confidence`}>
                      <div
                        className="confidence-fill"
                        style={{
                          width: `${ex.confidence * 100}%`,
                          backgroundColor: ex.confidence > 0.8 ? 'var(--accent2)' : 'var(--warning)',
                        }}
                      />
                    </div>
                  </td>
                  <td>
                    {ex.email_id ? (
                      <button
                        className="view-email-btn"
                        onClick={() => onSelectEmail(ex.email_id!)}
                      >
                        View Email
                      </button>
                    ) : (
                      <span className="muted">—</span>
                    )}
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} className="table-empty">
                    No transactions match the current filters.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}

function SaaSView({
  subscriptions,
  summary,
  formatMoney,
  onSelectEmail,
  saasSearch,
  setSaasSearch,
  saasStatus,
  setSaasStatus,
  sortField,
  sortOrder,
  onSort,
}: {
  subscriptions: Subscription[]
  summary: SaaSSummary | null
  formatMoney: (n: number, c?: string) => string
  onSelectEmail: (id: string) => void
  saasSearch: string
  setSaasSearch: (s: string) => void
  saasStatus: string
  setSaasStatus: (s: string) => void
  sortField: SortField
  sortOrder: SortOrder
  onSort: (field: SortField) => void
}) {
  const filtered = subscriptions
    .filter((sub) => {
      const matchSearch = sub.service_name.toLowerCase().includes(saasSearch.toLowerCase())
      const matchStatus = saasStatus === '' || sub.status.toLowerCase() === saasStatus.toLowerCase()
      return matchSearch && matchStatus
    })
    .sort((a, b) => {
      let valA: any = a[sortField === 'amount' ? 'amount' : sortField === 'confidence' ? 'confidence' : 'service_name']
      let valB: any = b[sortField === 'amount' ? 'amount' : sortField === 'confidence' ? 'confidence' : 'service_name']

      if (sortField === 'amount') {
        valA = a.amount ?? 0
        valB = b.amount ?? 0
      }

      if (valA < valB) return sortOrder === 'asc' ? -1 : 1
      if (valA > valB) return sortOrder === 'asc' ? 1 : -1
      return 0
    })

  return (
    <div className="dashboard-grid">
      <section className="stats-cards-row">
        <div className="stat-card">
          <span className="stat-icon" aria-hidden="true">A</span>
          <div>
            <span className="stat-label">Active Subscriptions</span>
            <span className="stat-value">{summary?.active_count ?? 0}</span>
          </div>
        </div>

        <div className="stat-card">
          <span className="stat-icon" aria-hidden="true">$</span>
          <div>
            <span className="stat-label">Monthly Estimate</span>
            <span className="stat-value">
              {summary ? formatMoney(summary.estimated_monthly, summary.currency) : '$0.00'}
            </span>
          </div>
        </div>

        <div className="stat-card">
          <span className="stat-icon" aria-hidden="true">T</span>
          <div>
            <span className="stat-label">Tracked Services</span>
            <span className="stat-value">{summary?.services.length ?? 0}</span>
          </div>
        </div>
      </section>

      <section className="card">
        <div className="card-header">
          <h2>Billing Cycles</h2>
          <span className="subtitle">Renewal cadence by detected service</span>
        </div>
        <div className="cycle-distribution">
          {summary &&
            Object.entries(summary.by_billing_cycle).map(([cycle, count]) => {
              const total = Object.values(summary.by_billing_cycle).reduce((a, b) => a + b, 0)
              const percentage = total > 0 ? (count / total) * 100 : 0
              return (
                <div key={cycle} className="cycle-distribution-row">
                  <div className="cycle-details">
                    <span className="cycle-name">{cycle || 'unknown'}</span>
                    <span className="cycle-count">{count} {count === 1 ? 'tool' : 'tools'}</span>
                  </div>
                  <div className="cycle-bar-track">
                    <div
                      className="cycle-bar-fill"
                      style={{
                        width: `${percentage}%`,
                      }}
                    />
                  </div>
                </div>
              )
            })}
          {(!summary || Object.keys(summary.by_billing_cycle).length === 0) && (
            <p className="empty">Upload sample emails to see distribution.</p>
          )}
        </div>
      </section>

      <section className="card wide">
        <div className="card-header list-header">
          <div>
            <h2>Discovered SaaS Tools</h2>
            <span className="subtitle">Showing {filtered.length} of {subscriptions.length} services</span>
          </div>

          <div className="filter-bar">
            <input
              type="text"
              placeholder="Search services"
              className="search-input"
              value={saasSearch}
              onChange={(e) => setSaasSearch(e.target.value)}
            />
            <select
              className="filter-select"
              value={saasStatus}
              onChange={(e) => setSaasStatus(e.target.value)}
            >
              <option value="">All Statuses</option>
              <option value="active">Active</option>
              <option value="cancelled">Cancelled</option>
              <option value="trial">Trial</option>
            </select>

            <div className="sorting-btns">
              <button
                className={`sort-btn ${sortField === 'service_name' ? 'active' : ''}`}
                onClick={() => onSort('service_name')}
              >
                Name {sortField === 'service_name' ? (sortOrder === 'asc' ? '▲' : '▼') : ''}
              </button>
              <button
                className={`sort-btn ${sortField === 'amount' ? 'active' : ''}`}
                onClick={() => onSort('amount')}
              >
                Cost {sortField === 'amount' ? (sortOrder === 'asc' ? '▲' : '▼') : ''}
              </button>
              <button
                className={`sort-btn ${sortField === 'confidence' ? 'active' : ''}`}
                onClick={() => onSort('confidence')}
              >
                Confidence {sortField === 'confidence' ? (sortOrder === 'asc' ? '▲' : '▼') : ''}
              </button>
            </div>
          </div>
        </div>

        <div className="saas-cards-grid">
          {filtered.map((sub) => (
            <article key={sub.id} className="subscription-card">
              <header className="saas-card-header">
                <div>
                  <h3>{sub.service_name}</h3>
                  <p className="plan-name">{sub.plan || 'Plan unspecified'}</p>
                </div>
                <span className={`status-pill ${sub.status.toLowerCase()}`}>{sub.status}</span>
              </header>

              <div className="saas-card-body">
                <span className="meta-label">Estimated price</span>
                <p className="amount-price">
                  {sub.amount != null ? formatMoney(sub.amount, sub.currency) : '—'}
                  {sub.billing_cycle && <span className="cycle-txt"> / {sub.billing_cycle}</span>}
                </p>

                <div className="confidence-section">
                  <div className="conf-lbl-row">
                    <span>Signal confidence</span>
                    <span>{(sub.confidence * 100).toFixed(0)}%</span>
                  </div>
                  <div className="conf-bar">
                    <div
                      className="conf-bar-fill"
                      style={{ width: `${sub.confidence * 100}%` }}
                    />
                  </div>
                </div>
              </div>

              <footer className="saas-card-footer">
                <span className="signal-badge" title="Detected from this email">{sub.signal_type}</span>
                {sub.email_id ? (
                  <button
                    className="view-email-btn secondary"
                    onClick={() => onSelectEmail(sub.email_id!)}
                  >
                    View source
                  </button>
                ) : (
                  <span className="muted">—</span>
                )}
              </footer>
            </article>
          ))}
          {filtered.length === 0 && (
            <div className="saas-empty-grid">No subscriptions match the current filters.</div>
          )}
        </div>
      </section>
    </div>
  )
}

function EmailDrawer({
  emailId,
  onClose,
}: {
  emailId: string
  onClose: () => void
}) {
  const [email, setEmail] = useState<Email | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    async function load() {
      setLoading(true)
      setError(null)
      try {
        const data = await api.email(emailId)
        if (active) setEmail(data)
      } catch (err) {
        if (active) setError(err instanceof Error ? err.message : 'Failed to load email')
      } finally {
        if (active) setLoading(false)
      }
    }
    load()
    return () => {
      active = false
    }
  }, [emailId])

  const renderHighlightedBody = (body: string) => {
    if (!body) return ''
    const amountRegex = /(\$\d+(?:\.\d{2})?)/g
    const parts = body.split(amountRegex)
    return parts.map((part, idx) => {
      if (amountRegex.test(part)) {
        return (
          <mark key={idx} className="highlight-price">
            {part}
          </mark>
        )
      }
      return part
    })
  }

  return (
    <div className="drawer-overlay" onClick={onClose}>
      <div className="drawer-container" onClick={(e) => e.stopPropagation()}>
        <header className="drawer-header">
          <div>
            <h3>Source Email</h3>
            <p className="drawer-subtitle">Original message used for extraction</p>
          </div>
          <button className="close-btn" onClick={onClose}>
            &times;
          </button>
        </header>

        <div className="drawer-body">
          {loading ? (
            <div className="drawer-loading">
              <div className="spinner"></div>
              <p>Loading original email...</p>
            </div>
          ) : error ? (
            <div className="drawer-error">
              <p>{error}</p>
            </div>
          ) : email ? (
            <div className="drawer-email-view">
              <div className="email-meta-box">
                <div className="meta-item">
                  <span className="lbl">Subject:</span>
                  <span className="val subject">{email.subject}</span>
                </div>
                <div className="meta-item">
                  <span className="lbl">From:</span>
                  <span className="val">{email.sender}</span>
                </div>
                <div className="meta-item">
                  <span className="lbl">To:</span>
                  <span className="val">{email.recipient}</span>
                </div>
                {email.received_at && (
                  <div className="meta-item">
                    <span className="lbl">Received:</span>
                    <span className="val">
                      {new Date(email.received_at).toLocaleString()}
                    </span>
                  </div>
                )}
                <div className="meta-item">
                  <span className="lbl">Classification:</span>
                  <span className={`lbl-classification ${email.email_type}`}>
                    {email.email_type}
                  </span>
                </div>
              </div>

              <div className="email-content-block">
                <h4>Email body</h4>
                <div className="email-body-box">
                  <pre>{renderHighlightedBody(email.body)}</pre>
                </div>
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}
