package parser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/waiyanphyoaung/email-insights/internal/models"
)

func ParseUpload(r io.Reader, filename string) ([]models.UploadEmailInput, error) {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".csv") {
		return parseCSV(r)
	}
	return parseJSON(r)
}

func parseJSON(r io.Reader) ([]models.UploadEmailInput, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var emails []models.UploadEmailInput
	if err := json.Unmarshal(data, &emails); err == nil {
		return normalize(emails), nil
	}

	var wrapper struct {
		Emails []models.UploadEmailInput `json:"emails"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("invalid JSON: expected array or {emails: [...]}")
	}
	return normalize(wrapper.Emails), nil
}

func parseCSV(r io.Reader) ([]models.UploadEmailInput, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have header and at least one row")
	}

	header := make(map[string]int)
	for i, col := range records[0] {
		header[strings.ToLower(strings.TrimSpace(col))] = i
	}

	get := func(row []string, keys ...string) string {
		for _, key := range keys {
			if idx, ok := header[key]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
		}
		return ""
	}

	var emails []models.UploadEmailInput
	for _, row := range records[1:] {
		emails = append(emails, models.UploadEmailInput{
			ID:        get(row, "id", "external_id"),
			Subject:   get(row, "subject"),
			From:      firstNonEmpty(get(row, "from", "sender"), get(row, "sender")),
			To:        firstNonEmpty(get(row, "to", "recipient"), get(row, "recipient")),
			Body:      get(row, "body", "content", "text"),
			Date:      firstNonEmpty(get(row, "date", "received_at", "receivedat")),
		})
	}
	return normalize(emails), nil
}

func normalize(emails []models.UploadEmailInput) []models.UploadEmailInput {
	out := make([]models.UploadEmailInput, 0, len(emails))
	for _, e := range emails {
		if e.From == "" {
			e.From = e.Sender
		}
		if e.To == "" {
			e.To = e.Recipient
		}
		if e.Date == "" {
			e.Date = e.ReceivedAt
		}
		if strings.TrimSpace(e.Subject) == "" && strings.TrimSpace(e.Body) == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func ParseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"01/02/2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return &t
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
