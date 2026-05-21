package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/waiyanphyoaung/email-insights/internal/models"
)

type OpenAIAnalyzer struct {
	client *openai.Client
	model  string
	fallback *MockAnalyzer
}

func NewOpenAI(apiKey, model string) *OpenAIAnalyzer {
	return &OpenAIAnalyzer{
		client:   openai.NewClient(apiKey),
		model:    model,
		fallback: NewMock(),
	}
}

const systemPrompt = `You analyze emails for personal finance insights.
Classify each email and extract structured data.

Email types: receipt, invoice, welcome, renewal, other, unknown

For receipts/orders: extract expense with merchant, amount, currency (ISO), date (YYYY-MM-DD), category.
Categories: Food & Dining, Shopping, Transport, Software & SaaS, Entertainment, Utilities, Healthcare, Travel, Other

For SaaS signals (welcome, invoice, renewal from software vendors): extract subscription with service_name, plan, amount, currency, billing_cycle (monthly/annual/quarterly), status (active/cancelled/trial), signal_type (welcome/invoice/renewal).

Return ONLY valid JSON:
{
  "email_type": "receipt",
  "expense": { "merchant": "", "amount": 0, "currency": "USD", "date": "", "category": "Other", "confidence": 0.9 } or null,
  "subscription": { "service_name": "", "plan": "", "amount": null, "currency": "USD", "billing_cycle": "", "status": "active", "signal_type": "invoice", "confidence": 0.9 } or null
}`

func (o *OpenAIAnalyzer) Analyze(ctx context.Context, subject, sender, body string) (*EmailAnalysis, error) {
	truncated := body
	if len(truncated) > 6000 {
		truncated = truncated[:6000] + "\n...[truncated]"
	}

	userContent := fmt.Sprintf("Subject: %s\nFrom: %s\n\nBody:\n%s", subject, sender, truncated)

	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: o.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userContent},
		},
		Temperature: 0.1,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return o.fallback.Analyze(ctx, subject, sender, body)
	}

	if len(resp.Choices) == 0 {
		return o.fallback.Analyze(ctx, subject, sender, body)
	}

	var raw struct {
		EmailType    string                `json:"email_type"`
		Expense      *ExpenseExtract       `json:"expense"`
		Subscription *SubscriptionExtract  `json:"subscription"`
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return o.fallback.Analyze(ctx, subject, sender, body)
	}

	return &EmailAnalysis{
		EmailType:    models.EmailType(raw.EmailType),
		Expense:      raw.Expense,
		Subscription: raw.Subscription,
	}, nil
}
