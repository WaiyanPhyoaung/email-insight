package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/waiyanphyoaung/email-insights/internal/analyzer"
	"github.com/waiyanphyoaung/email-insights/internal/models"
	"github.com/waiyanphyoaung/email-insights/internal/parser"
	"github.com/waiyanphyoaung/email-insights/internal/store"
	"github.com/google/uuid"
)

type Processor struct {
	store    *store.Store
	analyzer analyzer.Analyzer
}

func NewProcessor(st *store.Store, a analyzer.Analyzer) *Processor {
	return &Processor{store: st, analyzer: a}
}

func (p *Processor) ProcessUpload(ctx context.Context, inputs []models.UploadEmailInput) (*models.UploadResult, error) {
	result := &models.UploadResult{}

	for _, input := range inputs {
		receivedAt := parser.ParseDate(input.Date)
		analysis, err := p.analyzer.Analyze(ctx, input.Subject, input.From, input.Body)
		if err != nil {
			return nil, err
		}

		emailType := models.EmailTypeUnknown
		if analysis != nil {
			emailType = analysis.EmailType
		}

		extID := input.ID
		if extID == "" {
			h := sha256.New()
			h.Write([]byte(input.From + "|" + input.To + "|" + input.Subject + "|" + input.Date + "|" + input.Body))
			extID = fmt.Sprintf("hash-%x", h.Sum(nil))
		}

		email := models.Email{
			ExternalID:  extID,
			Subject:     input.Subject,
			Sender:      input.From,
			Recipient:   input.To,
			Body:        input.Body,
			ReceivedAt:  receivedAt,
			EmailType:   emailType,
			ProcessedAt: time.Now().UTC(),
		}

		emailID, err := p.store.InsertEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if emailID == uuid.Nil {
			// Skip duplicate email processing
			continue
		}
		result.EmailsProcessed++

		if analysis != nil && analysis.Expense != nil && analysis.Expense.Amount > 0 {
			var expenseDate *time.Time
			if analysis.Expense.Date != "" {
				expenseDate = parser.ParseDate(analysis.Expense.Date)
			}
			if expenseDate == nil {
				expenseDate = receivedAt
			}
			ex := models.Expense{
				EmailID:    &emailID,
				Merchant:   analysis.Expense.Merchant,
				Amount:     analysis.Expense.Amount,
				Currency:   defaultCurrency(analysis.Expense.Currency),
				ExpenseDate: expenseDate,
				Category:   analysis.Expense.Category,
				Confidence: analysis.Expense.Confidence,
			}
			if ex.Merchant == "" {
				ex.Merchant = "Unknown"
			}
			if ex.Category == "" {
				ex.Category = "Other"
			}
			if err := p.store.InsertExpense(ctx, ex); err != nil {
				return nil, err
			}
			result.ExpensesExtracted++
		}

		if analysis != nil && analysis.Subscription != nil && analysis.Subscription.ServiceName != "" {
			sub := models.Subscription{
				EmailID:      &emailID,
				ServiceName:  analysis.Subscription.ServiceName,
				VendorEmail:  input.From,
				Plan:         analysis.Subscription.Plan,
				Amount:       analysis.Subscription.Amount,
				Currency:     defaultCurrency(analysis.Subscription.Currency),
				BillingCycle: analysis.Subscription.BillingCycle,
				Status:       defaultStatus(analysis.Subscription.Status),
				SignalType:   analysis.Subscription.SignalType,
				Confidence:   analysis.Subscription.Confidence,
			}
			if sub.SignalType == "" {
				sub.SignalType = string(emailType)
			}
			if err := p.store.UpsertSubscription(ctx, sub); err != nil {
				return nil, err
			}
			result.SubscriptionsFound++
		}
	}

	return result, nil
}

func defaultCurrency(c string) string {
	if c == "" {
		return "USD"
	}
	return c
}

func defaultStatus(s string) string {
	if s == "" {
		return "active"
	}
	return s
}
