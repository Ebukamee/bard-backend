package email

import (
	"fmt"

	"github.com/resendlabs/resend-go"
)

// Mailer sends emails via Resend
type Mailer struct {
	client     *resend.Client
	fromAddr   string // e.g. "Bard <noreply@yourdomain.com>"
	appBaseURL string // e.g. "https://yourapp.com"
}

// NewMailer creates a Mailer with your Resend API key
func NewMailer(apiKey, fromAddr, appBaseURL string) *Mailer {
	client := resend.NewClient(apiKey)

	return &Mailer{
		client:     client,
		fromAddr:   fromAddr,
		appBaseURL: appBaseURL,
	}
}

// SendMagicLink sends a magic link email to the user.
// The rawToken is the unhashed token that goes in the URL.
func (m *Mailer) SendMagicLink(toEmail, rawToken string) error {
	magicLinkURL := fmt.Sprintf("%s/auth/verify?token=%s", m.appBaseURL, rawToken)

	html := fmt.Sprintf(`
		<div style="font-family: sans-serif; max-width: 480px; margin: 0 auto; padding: 24px;">
			<h2 style="color: #111;">Sign in to Bard</h2>
			<p style="color: #555; font-size: 16px;">
				Click the button below to sign in. This link expires in 15 minutes.
			</p>
			<a href="%s"
			   style="display: inline-block; background: #111; color: #fff;
			          padding: 12px 24px; border-radius: 6px; text-decoration: none;
			          font-size: 16px; margin: 16px 0;">
				Sign In
			</a>
			<p style="color: #999; font-size: 13px;">
				If you didn't request this, just ignore this email.
			</p>
			<p style="color: #999; font-size: 13px;">
				Or copy this link: %s
			</p>
		</div>
	`, magicLinkURL, magicLinkURL)

	params := &resend.SendEmailRequest{
		From:    m.fromAddr,
		To:      []string{toEmail},
		Subject: "Sign in to Bard",
		Html:    html,
	}

	_, err := m.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send magic link email: %w", err)
	}

	return nil
}
