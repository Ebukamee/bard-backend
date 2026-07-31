package email

import (
	"bytes"
	"fmt"
	"html/template"

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

var magicLinkTmpl = template.Must(template.New("magic-link").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Sign in to Bard</title>
  <link href="https://fonts.cdnfonts.com/css/karu" rel="stylesheet" />
  <link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@700&display=swap" rel="stylesheet" />
</head>
<body style="margin: 0; padding: 0; background-color: #040404; font-family: 'Karu', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color: #040404; padding: 40px 20px;">
    <tr>
      <td align="center">
        <table role="presentation" width="460" cellpadding="0" cellspacing="0" style="max-width: 460px; width: 100%;">

          <!-- Logo -->
          <tr>
            <td align="center" style="padding-bottom: 32px;">
              <span style="font-family: 'Space Grotesk', sans-serif; font-size: 28px; font-weight: 700; color: #ffffff; letter-spacing: 2px; text-transform: uppercase;">BARD</span>
            </td>
          </tr>

          <!-- Card -->
          <tr>
            <td style="background-color: #161616; border-radius: 16px; border: 1px solid rgba(255,255,255,0.05); padding: 40px 36px;">

              <!-- Heading -->
              <h1 style="margin: 0 0 8px; font-size: 22px; font-weight: 600; color: #ffffff; text-align: center; font-family: 'Karu', 'Space Grotesk', sans-serif;">
                Sign in to Bard
              </h1>
              <p style="margin: 0 0 32px; font-size: 14px; color: rgba(255,255,255,0.4); text-align: center; line-height: 1.5;">
                Click the button below to securely sign in.<br/>This link expires in 15 minutes.
              </p>

              <!-- Button -->
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td align="center">
                    <a href="{{.MagicLinkURL}}"
                       target="_blank"
                       style="display: inline-block; background-color: #ffffff; color: #040404; font-size: 14px; font-weight: 600; text-decoration: none; padding: 14px 40px; border-radius: 999px; letter-spacing: 0.2px;">
                      Sign in
                    </a>
                  </td>
                </tr>
              </table>

              <!-- Divider -->
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin: 32px 0;">
                <tr>
                  <td style="border-top: 1px solid rgba(255,255,255,0.06);"></td>
                </tr>
              </table>

              <!-- Fallback link -->
              <p style="margin: 0; font-size: 12px; color: rgba(255,255,255,0.25); text-align: center; line-height: 1.6;">
                If the button doesn't work, copy and paste this link:
              </p>
              <p style="margin: 8px 0 0; font-size: 11px; color: rgba(255,255,255,0.2); text-align: center; word-break: break-all; line-height: 1.5;">
                {{.MagicLinkURL}}
              </p>

            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding-top: 32px; text-align: center;">
              <p style="margin: 0 0 6px; font-size: 11px; color: rgba(255,255,255,0.15); line-height: 1.5;">
                You received this email because someone requested a sign-in link for your Bard account.
              </p>
              <p style="margin: 0; font-size: 11px; color: rgba(255,255,255,0.1);">
                If you didn't request this, you can safely ignore it.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

// SendMagicLink sends a magic link email to the user.
// The rawToken is the unhashed token that goes in the URL.
func (m *Mailer) SendMagicLink(toEmail, rawToken string) error {
	magicLinkURL := fmt.Sprintf("%s/auth/verify?token=%s", m.appBaseURL, rawToken)

	var buf bytes.Buffer
	err := magicLinkTmpl.Execute(&buf, map[string]string{
		"BaseURL":      m.appBaseURL,
		"MagicLinkURL": magicLinkURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	params := &resend.SendEmailRequest{
		From:    m.fromAddr,
		To:      []string{toEmail},
		Subject: "Sign in to Bard",
		Html:    buf.String(),
	}

	_, err = m.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send magic link email: %w", err)
	}

	return nil
}
