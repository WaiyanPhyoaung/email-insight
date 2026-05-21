package analyzer

import (
	"context"

	"github.com/waiyanphyoaung/email-insights/internal/models"
)

type EmailAnalysis struct {
	EmailType    models.EmailType
	Expense      *ExpenseExtract
	Subscription *SubscriptionExtract
}

type ExpenseExtract struct {
	Merchant   string  `json:"merchant"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	Date       string  `json:"date"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

type SubscriptionExtract struct {
	ServiceName  string   `json:"service_name"`
	Plan         string   `json:"plan"`
	Amount       *float64 `json:"amount"`
	Currency     string   `json:"currency"`
	BillingCycle string   `json:"billing_cycle"`
	Status       string   `json:"status"`
	SignalType   string   `json:"signal_type"`
	Confidence   float64  `json:"confidence"`
}

type Analyzer interface {
	Analyze(ctx context.Context, subject, sender, body string) (*EmailAnalysis, error)
}
