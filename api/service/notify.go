package service

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/SemRels/semrel-registry/api/models"
)

// ReviewNotifier delivers the outcome of a submission review to its author.
type ReviewNotifier interface {
	NotifyReview(ctx context.Context, notification models.ReviewNotification) error
}

// NoopNotifier is used when no delivery channel is configured. Notification is
// a convenience, not a correctness requirement: a registry without SMTP still
// records the outcome and shows it on the author's plugin list.
type NoopNotifier struct{}

func (NoopNotifier) NotifyReview(context.Context, models.ReviewNotification) error { return nil }

// SMTPConfig describes the mail relay used for review notifications.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// AppBaseURL is used to link the author back to their plugin list.
	AppBaseURL string
}

// Configured reports whether enough is set to attempt a delivery.
func (c SMTPConfig) Configured() bool {
	return strings.TrimSpace(c.Host) != "" && strings.TrimSpace(c.From) != ""
}

// SMTPNotifier sends review outcomes over SMTP.
type SMTPNotifier struct {
	cfg SMTPConfig
}

func NewSMTPNotifier(cfg SMTPConfig) *SMTPNotifier {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	return &SMTPNotifier{cfg: cfg}
}

// emailPattern is a deliberately loose check. Its job is to reject obvious
// nonsense and anything containing a newline — a header-injection attempt —
// not to decide which addresses the internet considers valid.
var emailPattern = regexp.MustCompile(`^[^\s@<>,;:"\\]+@[^\s@<>,;:"\\]+\.[A-Za-z]{2,}$`)

// ValidateNotifyEmail checks an opt-in contact address.
func ValidateNotifyEmail(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil // optional
	}
	if len(address) > 254 {
		return fmt.Errorf("address is too long")
	}
	if !emailPattern.MatchString(address) {
		return fmt.Errorf("does not look like an email address")
	}
	return nil
}

func (n *SMTPNotifier) NotifyReview(ctx context.Context, notification models.ReviewNotification) error {
	recipient := strings.TrimSpace(notification.Recipient)
	if recipient == "" {
		return nil
	}
	if err := ValidateNotifyEmail(recipient); err != nil {
		return fmt.Errorf("refusing to send to %q: %w", recipient, err)
	}

	subject, body := reviewMessage(notification, n.cfg.AppBaseURL)
	message := buildMessage(n.cfg.From, recipient, subject, body)

	addr := net.JoinHostPort(n.cfg.Host, fmt.Sprint(n.cfg.Port))
	var auth smtp.Auth
	if n.cfg.Username != "" {
		auth = smtp.PlainAuth("", n.cfg.Username, n.cfg.Password, n.cfg.Host)
	}

	// smtp.SendMail has no context support, so the deadline is enforced by
	// running it alongside one: a hung relay must not hold the review request.
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, n.cfg.From, []string{recipient}, message)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Second):
		return fmt.Errorf("smtp: sending to %s timed out", addr)
	}
}

// reviewMessage renders the subject and body. Plain text on purpose: the
// content is three sentences and a link, and an HTML mail would only add a
// rendering surface for the reviewer-supplied reason.
func reviewMessage(n models.ReviewNotification, appBaseURL string) (subject, body string) {
	pluginList := strings.TrimSuffix(appBaseURL, "/") + "/admin/plugins"

	if n.Approved {
		subject = fmt.Sprintf("%s is now published in the semrel registry", n.PluginRef)
		body = fmt.Sprintf(
			"Your plugin %s has been approved and is now listed in the semrel registry.\n\n"+
				"It can be installed with:\n\n    semrel plugin install %s\n\n"+
				"You can manage it at %s\n",
			n.PluginRef, n.PluginRef, pluginList)
		if n.Reason != "" {
			body += "\nNote from the reviewer:\n\n" + indent(n.Reason) + "\n"
		}
		return subject, body
	}

	subject = fmt.Sprintf("%s was not accepted into the semrel registry", n.PluginRef)
	body = fmt.Sprintf(
		"Your submission %s was reviewed and not accepted.\n\nReason given:\n\n%s\n\n"+
			"You can address this and submit again — the entry stays visible to you at %s\n",
		n.PluginRef, indent(n.Reason), pluginList)
	return subject, body
}

func indent(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

// buildMessage assembles the RFC 5322 message.
//
// Every header value is stripped of CR and LF: the plugin reference and the
// reviewer's reason both reach here from user input, and a newline inside a
// header is how an attacker appends headers or a second message body.
func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: " + sanitizeHeader(from) + "\r\n")
	b.WriteString("To: " + sanitizeHeader(to) + "\r\n")
	b.WriteString("Subject: " + sanitizeHeader(subject) + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String())
}

func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

// LoggingNotifier records what would have been sent. It is what a development
// environment gets instead of a mail relay, so the path is exercised without
// anyone receiving anything.
type LoggingNotifier struct{}

func (LoggingNotifier) NotifyReview(_ context.Context, n models.ReviewNotification) error {
	outcome := "rejected"
	if n.Approved {
		outcome = "approved"
	}
	log.Printf("review notification (not sent — no SMTP configured): %s %s → %s", n.PluginRef, outcome, n.Recipient)
	return nil
}
