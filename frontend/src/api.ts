const API_BASE = import.meta.env.VITE_API_URL ?? ''

export interface Expense {
  id: string
  merchant: string
  amount: number
  currency: string
  expense_date?: string
  category: string
  confidence: number
}

export interface SpendingSummary {
  total_amount: number
  currency: string
  transaction_count: number
  by_category: Record<string, number>
  top_merchants: { merchant: string; amount: number; count: number }[]
}

export interface Subscription {
  id: string
  service_name: string
  vendor_email?: string
  plan?: string
  amount?: number
  currency?: string
  billing_cycle?: string
  status: string
  signal_type: string
  confidence: number
}

export interface SaaSSummary {
  active_count: number
  estimated_monthly: number
  currency: string
  by_billing_cycle: Record<string, number>
  services: {
    service_name: string
    status: string
    amount?: number
    currency?: string
    billing_cycle?: string
    signal_count: number
  }[]
}

export interface UploadResult {
  emails_processed: number
  expenses_extracted: number
  subscriptions_found: number
}

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error((err as { error?: string }).error ?? 'Request failed')
  }
  return res.json()
}

export const api = {
  spending: () => fetchJSON<Expense[]>('/api/spending'),
  spendingSummary: () => fetchJSON<SpendingSummary>('/api/spending/summary'),
  saas: () => fetchJSON<Subscription[]>('/api/saas'),
  saasSummary: () => fetchJSON<SaaSSummary>('/api/saas/summary'),
  upload: async (file: File): Promise<UploadResult> => {
    const form = new FormData()
    form.append('file', file)
    const res = await fetch(`${API_BASE}/api/emails/upload`, { method: 'POST', body: form })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error((err as { error?: string }).error ?? 'Upload failed')
    }
    return res.json()
  },
}
