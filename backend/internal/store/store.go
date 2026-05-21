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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO emails (id, external_id, subject, sender, recipient, body, received_at, email_type, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, id, nullStr(e.ExternalID), e.Subject, e.Sender, e.Recipient, e.Body, e.ReceivedAt, string(e.EmailType), e.ProcessedAt)
	return id, err
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
	existing, err := s.findSubscriptionByService(ctx, sub.ServiceName)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if existing != nil {
		amount := sub.Amount
		if amount == nil {
			amount = existing.Amount
		}
		plan := sub.Plan
		if plan == "" {
			plan = existing.Plan
		}
		cycle := sub.BillingCycle
		if cycle == "" {
			cycle = existing.BillingCycle
		}
		_, err = s.pool.Exec(ctx, `
			UPDATE subscriptions SET
				email_id = COALESCE($2, email_id),
				vendor_email = COALESCE(NULLIF($3, ''), vendor_email),
				plan = COALESCE(NULLIF($4, ''), plan),
				amount = COALESCE($5, amount),
				currency = COALESCE(NULLIF($6, ''), currency),
				billing_cycle = COALESCE(NULLIF($7, ''), billing_cycle),
				status = $8,
				signal_type = $9,
				confidence = GREATEST(confidence, $10),
				last_seen_at = $11
			WHERE id = $1
		`, existing.ID, sub.EmailID, sub.VendorEmail, plan, amount, sub.Currency, cycle,
			sub.Status, sub.SignalType, sub.Confidence, now)
		return err
	}

	id := sub.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO subscriptions (id, email_id, service_name, vendor_email, plan, amount, currency, billing_cycle, status, signal_type, confidence, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
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
		summary.TotalAmount += ex.Amount
		summary.TransactionCount++
		summary.ByCategory[ex.Category] += ex.Amount
		if _, ok := merchantMap[ex.Merchant]; !ok {
			merchantMap[ex.Merchant] = &models.MerchantTotal{Merchant: ex.Merchant}
		}
		merchantMap[ex.Merchant].Amount += ex.Amount
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
			summary.EstimatedMonthly += monthly
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
