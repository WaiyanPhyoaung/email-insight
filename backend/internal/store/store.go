package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/waiyanphyoaung/email-insights/internal/models"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) RunMigrations(ctx context.Context, path string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, string(sqlBytes))
	return err
}

func (s *Store) InsertEmail(ctx context.Context, e models.Email) (uuid.UUID, error) {
	id := e.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	var insertedID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO emails (id, external_id, subject, sender, recipient, body, received_at, email_type, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (external_id) WHERE external_id IS NOT NULL DO NOTHING
		RETURNING id
	`, id, nullStr(e.ExternalID), e.Subject, e.Sender, e.Recipient, e.Body, e.ReceivedAt, string(e.EmailType), e.ProcessedAt).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}
	return insertedID, nil
}

func (s *Store) InsertExpense(ctx context.Context, ex models.Expense) error {
	id := ex.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO expenses (id, email_id, merchant, amount, currency, expense_date, category, confidence)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, ex.EmailID, ex.Merchant, ex.Amount, ex.Currency, ex.ExpenseDate, ex.Category, ex.Confidence)
	return err
}

func (s *Store) UpsertSubscription(ctx context.Context, sub models.Subscription) error {
	id := sub.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscriptions (id, email_id, service_name, vendor_email, plan, amount, currency, billing_cycle, status, signal_type, confidence, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (LOWER(service_name)) DO UPDATE SET
			email_id = COALESCE(EXCLUDED.email_id, subscriptions.email_id),
			vendor_email = COALESCE(NULLIF(EXCLUDED.vendor_email, ''), subscriptions.vendor_email),
			plan = COALESCE(NULLIF(EXCLUDED.plan, ''), subscriptions.plan),
			amount = COALESCE(EXCLUDED.amount, subscriptions.amount),
			currency = COALESCE(NULLIF(EXCLUDED.currency, ''), subscriptions.currency),
			billing_cycle = COALESCE(NULLIF(EXCLUDED.billing_cycle, ''), subscriptions.billing_cycle),
			status = EXCLUDED.status,
			signal_type = EXCLUDED.signal_type,
			confidence = GREATEST(subscriptions.confidence, EXCLUDED.confidence),
			last_seen_at = EXCLUDED.last_seen_at
	`, id, sub.EmailID, sub.ServiceName, sub.VendorEmail, sub.Plan, sub.Amount, sub.Currency,
		sub.BillingCycle, sub.Status, sub.SignalType, sub.Confidence, now, now)
	return err
}

func (s *Store) findSubscriptionByService(ctx context.Context, name string) (*models.Subscription, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email_id, service_name, vendor_email, plan, amount, currency, billing_cycle, status, signal_type, confidence, first_seen_at, last_seen_at, created_at
		FROM subscriptions WHERE LOWER(service_name) = LOWER($1) LIMIT 1
	`, name)

	var sub models.Subscription
	var emailID *uuid.UUID
	var amount *float64
	err := row.Scan(&sub.ID, &emailID, &sub.ServiceName, &sub.VendorEmail, &sub.Plan, &amount,
		&sub.Currency, &sub.BillingCycle, &sub.Status, &sub.SignalType, &sub.Confidence,
		&sub.FirstSeenAt, &sub.LastSeenAt, &sub.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	sub.EmailID = emailID
	sub.Amount = amount
	return &sub, nil
}

func (s *Store) ListExpenses(ctx context.Context) ([]models.Expense, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email_id, merchant, amount, currency, expense_date, category, confidence, created_at
		FROM expenses ORDER BY COALESCE(expense_date, created_at::date) DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.Expense, 0)
	for rows.Next() {
		var ex models.Expense
		var emailID *uuid.UUID
		var expenseDate *time.Time
		if err := rows.Scan(&ex.ID, &emailID, &ex.Merchant, &ex.Amount, &ex.Currency, &expenseDate,
			&ex.Category, &ex.Confidence, &ex.CreatedAt); err != nil {
			return nil, err
		}
		ex.EmailID = emailID
		ex.ExpenseDate = expenseDate
		out = append(out, ex)
	}
	return out, rows.Err()
}

func (s *Store) SpendingSummary(ctx context.Context) (*models.SpendingSummary, error) {
	expenses, err := s.ListExpenses(ctx)
	if err != nil {
		return nil, err
	}

	summary := &models.SpendingSummary{
		Currency:   "USD",
		ByCategory: make(map[string]float64),
	}
	merchantMap := make(map[string]*models.MerchantTotal)

	for _, ex := range expenses {
		usdAmount := convertToUSD(ex.Amount, ex.Currency)
		summary.TotalAmount += usdAmount
		summary.TransactionCount++
		summary.ByCategory[ex.Category] += usdAmount
		if _, ok := merchantMap[ex.Merchant]; !ok {
			merchantMap[ex.Merchant] = &models.MerchantTotal{Merchant: ex.Merchant}
		}
		merchantMap[ex.Merchant].Amount += usdAmount
		merchantMap[ex.Merchant].Count++
	}

	for _, m := range merchantMap {
		summary.TopMerchants = append(summary.TopMerchants, *m)
	}
	// simple sort by amount desc
	for i := 0; i < len(summary.TopMerchants); i++ {
		for j := i + 1; j < len(summary.TopMerchants); j++ {
			if summary.TopMerchants[j].Amount > summary.TopMerchants[i].Amount {
				summary.TopMerchants[i], summary.TopMerchants[j] = summary.TopMerchants[j], summary.TopMerchants[i]
			}
		}
	}
	if len(summary.TopMerchants) > 10 {
		summary.TopMerchants = summary.TopMerchants[:10]
	}
	return summary, nil
}

func (s *Store) ListSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email_id, service_name, vendor_email, plan, amount, currency, billing_cycle, status, signal_type, confidence, first_seen_at, last_seen_at, created_at
		FROM subscriptions ORDER BY service_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.Subscription, 0)
	for rows.Next() {
		var sub models.Subscription
		var emailID *uuid.UUID
		var amount *float64
		if err := rows.Scan(&sub.ID, &emailID, &sub.ServiceName, &sub.VendorEmail, &sub.Plan, &amount,
			&sub.Currency, &sub.BillingCycle, &sub.Status, &sub.SignalType, &sub.Confidence,
			&sub.FirstSeenAt, &sub.LastSeenAt, &sub.CreatedAt); err != nil {
			return nil, err
		}
		sub.EmailID = emailID
		sub.Amount = amount
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *Store) SaaSSummary(ctx context.Context) (*models.SaaSSummary, error) {
	subs, err := s.ListSubscriptions(ctx)
	if err != nil {
		return nil, err
	}

	summary := &models.SaaSSummary{
		Currency:       "USD",
		ByBillingCycle: make(map[string]int),
	}
	serviceMap := make(map[string]*models.SubscriptionRollup)

	for _, sub := range subs {
		if sub.Status == "active" {
			summary.ActiveCount++
		}
		cycle := sub.BillingCycle
		if cycle == "" {
			cycle = "unknown"
		}
		summary.ByBillingCycle[cycle]++

		if sub.Amount != nil {
			monthly := *sub.Amount
			switch strings.ToLower(sub.BillingCycle) {
			case "annual", "yearly":
				monthly /= 12
			case "quarterly":
				monthly /= 3
			}
			usdMonthly := convertToUSD(monthly, sub.Currency)
			summary.EstimatedMonthly += usdMonthly
		}

		key := strings.ToLower(sub.ServiceName)
		if _, ok := serviceMap[key]; !ok {
			serviceMap[key] = &models.SubscriptionRollup{
				ServiceName:  sub.ServiceName,
				Status:       sub.Status,
				Amount:       sub.Amount,
				Currency:     sub.Currency,
				BillingCycle: sub.BillingCycle,
			}
		}
		serviceMap[key].SignalCount++
	}

	for _, r := range serviceMap {
		summary.Services = append(summary.Services, *r)
	}
	return summary, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) Health(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func MigrationPath() string {
	candidates := []string{
		"migrations/001_init.sql",
		"/app/migrations/001_init.sql",
		"backend/migrations/001_init.sql",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

func WaitForDB(ctx context.Context, url string, attempts int) (*Store, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		st, err := New(ctx, url)
		if err == nil {
			if err = st.Health(ctx); err == nil {
				return st, nil
			}
			st.Close()
			lastErr = err
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("database not ready: %w", lastErr)
}

func convertToUSD(amount float64, currency string) float64 {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USD", "$":
		return amount
	case "EUR", "€":
		return amount * 1.08
	case "GBP", "£":
		return amount * 1.27
	case "CAD":
		return amount * 0.73
	case "AUD":
		return amount * 0.66
	case "JPY":
		return amount * 0.0064
	default:
		return amount
	}
}

func (s *Store) GetEmail(ctx context.Context, id uuid.UUID) (*models.Email, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, external_id, subject, sender, recipient, body, received_at, email_type, processed_at, created_at
		FROM emails WHERE id = $1
	`, id)
	var e models.Email
	var extID *string
	var receivedAt *time.Time
	err := row.Scan(&e.ID, &extID, &e.Subject, &e.Sender, &e.Recipient, &e.Body, &receivedAt, &e.EmailType, &e.ProcessedAt, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if extID != nil {
		e.ExternalID = *extID
	}
	e.ReceivedAt = receivedAt
	return &e, nil
}
