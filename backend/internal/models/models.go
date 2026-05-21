package models

import (
	"time"

	"github.com/google/uuid"
)

type EmailType string

const (
	EmailTypeReceipt   EmailType = "receipt"
	EmailTypeInvoice   EmailType = "invoice"
	EmailTypeWelcome   EmailType = "welcome"
	EmailTypeRenewal   EmailType = "renewal"
	EmailTypeUnknown   EmailType = "unknown"
	EmailTypeOther     EmailType = "other"
)

type Email struct {
	ID          uuid.UUID  `json:"id"`
	ExternalID  string     `json:"external_id,omitempty"`
	Subject     string     `json:"subject"`
	Sender      string     `json:"sender"`
	Recipient   string     `json:"recipient"`
	Body        string     `json:"body"`
	ReceivedAt  *time.Time `json:"received_at,omitempty"`
	EmailType   EmailType  `json:"email_type"`
	ProcessedAt time.Time  `json:"processed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Expense struct {
	ID         uuid.UUID  `json:"id"`
	EmailID    *uuid.UUID `json:"email_id,omitempty"`
	Merchant   string     `json:"merchant"`
	Amount     float64    `json:"amount"`
	Currency   string     `json:"currency"`
	ExpenseDate *time.Time `json:"expense_date,omitempty"`
	Category   string     `json:"category"`
	Confidence float64    `json:"confidence"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Subscription struct {
	ID           uuid.UUID  `json:"id"`
	EmailID      *uuid.UUID `json:"email_id,omitempty"`
	ServiceName  string     `json:"service_name"`
	VendorEmail  string     `json:"vendor_email,omitempty"`
	Plan         string     `json:"plan,omitempty"`
	Amount       *float64   `json:"amount,omitempty"`
	Currency     string     `json:"currency,omitempty"`
	BillingCycle string     `json:"billing_cycle,omitempty"`
	Status       string     `json:"status"`
	SignalType   string     `json:"signal_type"`
	Confidence   float64    `json:"confidence"`
	FirstSeenAt  time.Time  `json:"first_seen_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type SpendingSummary struct {
	TotalAmount      float64            `json:"total_amount"`
	Currency         string             `json:"currency"`
	TransactionCount int                `json:"transaction_count"`
	ByCategory       map[string]float64 `json:"by_category"`
	TopMerchants     []MerchantTotal    `json:"top_merchants"`
}

type MerchantTotal struct {
	Merchant string  `json:"merchant"`
	Amount   float64 `json:"amount"`
	Count    int     `json:"count"`
}

type SaaSSummary struct {
	ActiveCount      int                `json:"active_count"`
	EstimatedMonthly float64            `json:"estimated_monthly"`
	Currency         string             `json:"currency"`
	ByBillingCycle   map[string]int     `json:"by_billing_cycle"`
	Services         []SubscriptionRollup `json:"services"`
}

type SubscriptionRollup struct {
	ServiceName  string   `json:"service_name"`
	Status       string   `json:"status"`
	Amount       *float64 `json:"amount,omitempty"`
	Currency     string   `json:"currency,omitempty"`
	BillingCycle string   `json:"billing_cycle,omitempty"`
	SignalCount  int      `json:"signal_count"`
}

type UploadEmailInput struct {
	ID        string `json:"id"`
	Subject   string `json:"subject"`
	From      string `json:"from"`
	To        string `json:"to"`
	Body      string `json:"body"`
	Date      string `json:"date"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	ReceivedAt string `json:"received_at"`
}

type UploadResult struct {
	EmailsProcessed   int `json:"emails_processed"`
	ExpensesExtracted int `json:"expenses_extracted"`
	SubscriptionsFound int `json:"subscriptions_found"`
}
