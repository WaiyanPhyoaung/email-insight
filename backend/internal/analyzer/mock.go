package analyzer

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/waiyanphyoaung/email-insights/internal/models"
)

// MockAnalyzer uses heuristics when no OpenAI key is configured.
type MockAnalyzer struct{}

func NewMock() *MockAnalyzer {
	return &MockAnalyzer{}
}

var amountRe = regexp.MustCompile(`(?i)\$?\s*([\d,]+\.?\d{0,2})`)

func (m *MockAnalyzer) Analyze(_ context.Context, subject, sender, body string) (*EmailAnalysis, error) {
	combined := strings.ToLower(subject + " " + body)
	senderLower := strings.ToLower(sender)

	analysis := &EmailAnalysis{EmailType: models.EmailTypeUnknown}

	if isSaaSSignal(combined, subject) {
		analysis.EmailType = classifySaaSType(combined, subject)
		analysis.Subscription = extractSubscriptionHeuristic(subject, sender, body, analysis.EmailType)
	}

	if isReceipt(combined, subject) {
		if analysis.EmailType == models.EmailTypeUnknown {
			analysis.EmailType = models.EmailTypeReceipt
		}
		analysis.Expense = extractExpenseHeuristic(subject, sender, body)
	}

	if analysis.Subscription == nil && looksLikeVendor(senderLower) {
		analysis.Subscription = extractSubscriptionHeuristic(subject, sender, body, models.EmailTypeInvoice)
		if analysis.Subscription != nil {
			analysis.EmailType = models.EmailTypeInvoice
		}
	}

	return analysis, nil
}

func isSaaSSignal(combined, subject string) bool {
	keywords := []string{
		"welcome to", "thanks for subscribing", "subscription confirmed",
		"your invoice", "renewal", "billing receipt", "plan renewal",
		"trial started", "account created",
	}
	subjectLower := strings.ToLower(subject)
	for _, k := range keywords {
		if strings.Contains(combined, k) || strings.Contains(subjectLower, k) {
			return true
		}
	}
	saasDomains := []string{"notion.so", "slack.com", "figma.com", "github.com", "stripe.com",
		"openai.com", "anthropic.com", "linear.app", "vercel.com", "dropbox.com"}
	for _, d := range saasDomains {
		if strings.Contains(combined, d) {
			return true
		}
	}
	return false
}

func classifySaaSType(combined, subject string) models.EmailType {
	subjectLower := strings.ToLower(subject)
	switch {
	case strings.Contains(combined, "welcome") || strings.Contains(combined, "account created"):
		return models.EmailTypeWelcome
	case strings.Contains(combined, "renewal") || strings.Contains(subjectLower, "renew"):
		return models.EmailTypeRenewal
	default:
		return models.EmailTypeInvoice
	}
}

func isReceipt(combined, subject string) bool {
	keywords := []string{"receipt", "order confirmation", "payment received", "your purchase", "charged"}
	subjectLower := strings.ToLower(subject)
	for _, k := range keywords {
		if strings.Contains(combined, k) || strings.Contains(subjectLower, k) {
			return true
		}
	}
	return false
}

func looksLikeVendor(sender string) bool {
	return strings.Contains(sender, "billing") || strings.Contains(sender, "invoice") ||
		strings.Contains(sender, "noreply") && (strings.Contains(sender, ".com") || strings.Contains(sender, ".io"))
}

func extractExpenseHeuristic(subject, sender, body string) *ExpenseExtract {
	merchant := merchantFromSender(sender)
	if merchant == "" {
		merchant = merchantFromSubject(subject)
	}
	amount := firstAmount(body + " " + subject)
	if amount <= 0 {
		return nil
	}
	category := categorize(merchant, subject, body)
	return &ExpenseExtract{
		Merchant:   merchant,
		Amount:     amount,
		Currency:   "USD",
		Date:       "",
		Category:   category,
		Confidence: 0.65,
	}
}

func extractSubscriptionHeuristic(subject, sender, body string, emailType models.EmailType) *SubscriptionExtract {
	service := merchantFromSender(sender)
	if service == "" {
		service = merchantFromSubject(subject)
	}
	if service == "" {
		return nil
	}
	amount := firstAmount(body + " " + subject)
	var amtPtr *float64
	if amount > 0 {
		amtPtr = &amount
	}
	signal := string(emailType)
	if signal == string(models.EmailTypeUnknown) {
		signal = "invoice"
	}
	return &SubscriptionExtract{
		ServiceName:  service,
		Plan:         inferPlan(body),
		Amount:       amtPtr,
		Currency:     "USD",
		BillingCycle: inferBillingCycle(body),
		Status:       "active",
		SignalType:   signal,
		Confidence:   0.6,
	}
}

func merchantFromSender(sender string) string {
	if sender == "" {
		return ""
	}
	at := strings.Index(sender, "@")
	if at < 0 {
		return strings.TrimSpace(sender)
	}
	domain := sender[at+1:]
	domain = strings.TrimSuffix(domain, ">")
	parts := strings.Split(domain, ".")
	if len(parts) > 0 {
		name := strings.Title(parts[0])
		if name != "Noreply" && name != "Billing" && name != "Mail" {
			return name
		}
		if len(parts) > 1 {
			return strings.Title(parts[len(parts)-2])
		}
	}
	return ""
}

func merchantFromSubject(subject string) string {
	// "Your receipt from Amazon" -> Amazon
	lower := strings.ToLower(subject)
	for _, prefix := range []string{"receipt from ", "order from ", "invoice from "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			rest := subject[idx+len(prefix):]
			if space := strings.IndexAny(rest, " -:"); space > 0 {
				return strings.TrimSpace(rest[:space])
			}
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func firstAmount(text string) float64 {
	matches := amountRe.FindAllStringSubmatch(text, -1)
	var best float64
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.ReplaceAll(m[1], ",", "")
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 || v > 100000 {
			continue
		}
		if v > best {
			best = v
		}
	}
	return best
}

func categorize(merchant, subject, body string) string {
	combined := strings.ToLower(merchant + " " + subject + " " + body)
	rules := map[string][]string{
		"Food & Dining":    {"uber eats", "doordash", "grubhub", "restaurant", "coffee", "starbucks"},
		"Shopping":         {"amazon", "ebay", "walmart", "target", "order"},
		"Transport":        {"uber", "lyft", "gas", "parking"},
		"Software & SaaS":  {"subscription", "saas", "notion", "slack", "github", "figma"},
		"Entertainment":    {"netflix", "spotify", "hulu", "steam"},
		"Utilities":        {"electric", "water", "internet", "phone bill"},
	}
	for cat, keys := range rules {
		for _, k := range keys {
			if strings.Contains(combined, k) {
				return cat
			}
		}
	}
	return "Other"
}

func inferPlan(body string) string {
	lower := strings.ToLower(body)
	for _, plan := range []string{"pro", "business", "enterprise", "team", "premium", "plus"} {
		if strings.Contains(lower, plan+" plan") || strings.Contains(lower, plan+" subscription") {
			return strings.Title(plan)
		}
	}
	return ""
}

func inferBillingCycle(body string) string {
	lower := strings.ToLower(body)
	switch {
	case strings.Contains(lower, "annual") || strings.Contains(lower, "yearly"):
		return "annual"
	case strings.Contains(lower, "quarter"):
		return "quarterly"
	case strings.Contains(lower, "month"):
		return "monthly"
	default:
		return ""
	}
}
