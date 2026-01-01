package channels

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// EmailChannel sends alerts via SMTP
type EmailChannel struct {
	smtpHost string
	smtpPort int
	from     string
	to       []string
	auth     smtp.Auth // For authenticated SMTP (currently nil)
}

// NewEmailChannel creates a new email notification channel
func NewEmailChannel(smtpHost string, smtpPort int, from string, to []string) *EmailChannel {
	return &EmailChannel{
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		from:     from,
		to:       to,
		auth:     nil, // No authentication for now
	}
}

// Send delivers alert via email
func (e *EmailChannel) Send(alert Alert) error {
	if e.smtpHost == "" {
		return fmt.Errorf("smtp host not configured")
	}
	if len(e.to) == 0 {
		return fmt.Errorf("no email recipients configured")
	}

	subject := fmt.Sprintf("[FLIP2] %s: %s", strings.ToUpper(alert.GetSeverity()), alert.GetAlertName())
	body := e.formatMessage(alert)

	msg := formatEmailMessage(e.from, e.to, subject, body)

	addr := fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort)
	err := smtp.SendMail(addr, e.auth, e.from, e.to, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// formatMessage creates email body (plain text)
func (e *EmailChannel) formatMessage(alert Alert) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("FLIP2 Alert: %s\n", alert.GetAlertName()))
	sb.WriteString(fmt.Sprintf("Severity: %s\n\n", strings.ToUpper(alert.GetSeverity())))

	// Alert details
	sb.WriteString(fmt.Sprintf("Message: %s\n\n", alert.GetMessage()))
	sb.WriteString(fmt.Sprintf("Current Value: %s\n", formatEmailValue(alert.GetMetricValue())))
	sb.WriteString(fmt.Sprintf("Threshold: %s\n\n", formatEmailValue(alert.GetThreshold())))

	// Timestamp
	sb.WriteString(fmt.Sprintf("Fired At: %s\n\n", alert.GetFiredAt().Format(time.RFC1123)))

	// Metadata if present
	metadata := alert.GetMetadata()
	if len(metadata) > 0 {
		sb.WriteString("Additional Information:\n")
		for key, value := range metadata {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("---\n")
	sb.WriteString("This is an automated alert from FLIP2.\n")

	return sb.String()
}

// formatEmailValue formats a float64 value for email display
func formatEmailValue(value float64) string {
	// For whole numbers, don't show decimal places
	if value == float64(int64(value)) {
		return fmt.Sprintf("%.0f", value)
	}
	// For fractional numbers, show up to 2 decimal places
	return fmt.Sprintf("%.2f", value)
}

// formatEmailMessage creates a properly formatted email message with headers
func formatEmailMessage(from string, to []string, subject, body string) string {
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = strings.Join(to, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=utf-8"

	var msg strings.Builder
	for key, value := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}
