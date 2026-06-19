package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/wneessen/go-mail"
)

// emailConfig is the per-channel config JSON for email channels.
type emailConfig struct {
	To string `json:"to"`
}

// smtpSettings is the resolved, decrypted SMTP transport used to build a client.
// It deliberately holds the plaintext password - it is constructed transiently
// at send time and never persisted or logged.
type smtpSettings struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLSMode  SMTPTLSMode
}

// loadSMTPSettings reads the server-wide SMTP config, decrypts the password, and
// returns the transport settings. Returns an error if SMTP is not configured,
// not enabled, or the encryption key is unavailable.
func (s *Server) loadSMTPSettings(ctx context.Context) (smtpSettings, error) {
	cfg, err := s.DB.GetSMTPConfig(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return smtpSettings{}, errors.New("SMTP is not configured")
		}
		return smtpSettings{}, err
	}
	if !cfg.Enabled {
		return smtpSettings{}, errors.New("SMTP delivery is disabled")
	}

	tlsMode, err := ParseSMTPTLSMode(cfg.TlsMode)
	if err != nil {
		return smtpSettings{}, fmt.Errorf("invalid stored SMTP TLS mode: %w", err)
	}

	var password string
	if cfg.PasswordEncrypted != "" {
		if s.Cipher == nil {
			return smtpSettings{}, errors.New("SMTP password is stored but the encryption key is not configured")
		}
		password, err = s.Cipher.DecryptString(cfg.PasswordEncrypted)
		if err != nil {
			return smtpSettings{}, fmt.Errorf("decrypt SMTP password: %w", err)
		}
	}

	return smtpSettings{
		Host:     cfg.Host,
		Port:     int(cfg.Port),
		Username: cfg.Username,
		Password: password,
		From:     cfg.FromAddress,
		TLSMode:  tlsMode,
	}, nil
}

// sendEmail delivers an alert notification using the server-wide SMTP transport.
func (s *Server) sendEmail(ctx context.Context, ch database.AlertChannel, payload AlertPayload) error {
	var cfg emailConfig
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse email channel config: %w", err)
	}
	if strings.TrimSpace(cfg.To) == "" {
		return fmt.Errorf("email channel %q has no recipient", ch.Name)
	}

	settings, err := s.loadSMTPSettings(context.Background())
	if err != nil {
		return err
	}

	msg, err := buildAlertEmail(settings.From, cfg.To, payload)
	if err != nil {
		return err
	}
	return sendMail(ctx, settings, msg)
}

// sendTestEmail sends a canned message using the supplied settings, for the
// admin test-config endpoint. Settings come from the request, not the DB.
func (s *Server) sendTestEmail(ctx context.Context, settings smtpSettings, to string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return errors.New("test recipient is empty")
	}

	msg := mail.NewMsg()
	if err := msg.From(settings.From); err != nil {
		return fmt.Errorf("set from address %q: %w", settings.From, err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("set to address %q: %w", to, err)
	}
	msg.Subject("[Spectra] SMTP Test Message")
	msg.SetBodyString(mail.TypeTextPlain,
		"This is a test message from Spectra confirming your SMTP settings work.\n")

	return sendMail(ctx, settings, msg)
}

// buildAlertEmail constructs the go-mail message for an alert notification.
func buildAlertEmail(from, to string, p AlertPayload) (*mail.Msg, error) {
	msg := mail.NewMsg()
	if err := msg.From(from); err != nil {
		return nil, fmt.Errorf("set from address %q: %w", from, err)
	}
	if err := msg.To(to); err != nil {
		return nil, fmt.Errorf("set to address %q: %w", to, err)
	}

	msg.Subject(fmt.Sprintf("[Spectra] %s - %s", p.RuleName, p.ConditionType))

	var b strings.Builder
	fmt.Fprintf(&b, "Alert rule: %s\n", p.RuleName)
	fmt.Fprintf(&b, "Condition: %s\n", p.ConditionType)
	fmt.Fprintf(&b, "Agent: %s\n", p.AgentID)
	fmt.Fprintf(&b, "Fired at: %s\n", p.FiredAt)
	if p.Snapshot != nil {
		if snap, err := json.MarshalIndent(p.Snapshot, "", "  "); err == nil {
			fmt.Fprintf(&b, "\nDetails:\n%s\n", snap)
		}
	}

	msg.SetBodyString(mail.TypeTextPlain, b.String())
	return msg, nil
}

// sendMail builds a go-mail client from settings and delivers msg. Connection
// security follows settings.TLSMode; auth is applied only when a username is set.
func sendMail(ctx context.Context, settings smtpSettings, msg *mail.Msg) error {
	if settings.Host == "" {
		return errors.New("SMTP host is empty")
	}
	if settings.From == "" {
		return errors.New("SMTP port is invalid")
	}
	if settings.Port < 1 || settings.Port > 65535 {
		return errors.New("SMTP port is invalid")
	}

	opts := []mail.Option{
		mail.WithPort(settings.Port),
		mail.WithTimeout(10 * time.Second),
	}

	switch settings.TLSMode {
	case SMTPTLSImplicit:
		opts = append(opts, mail.WithSSL())
	case SMTPTLSStartTLS:
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case SMTPTLSNone:
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default:
		return fmt.Errorf("unknown SMTP TLS mode %q", settings.TLSMode)
	}

	if settings.Username != "" {
		if settings.Password == "" {
			return errors.New("SMTP username is set but password is empty")
		}
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(settings.Username),
			mail.WithPassword(settings.Password),
		)
	}

	client, err := mail.NewClient(settings.Host, opts...)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}
