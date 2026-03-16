// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package email

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strings"
)

// SMTPConfig holds the SMTP configuration sourced from environment variables.
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	UseTLS   bool
}

// LoadSMTPConfig reads SMTP settings from ST_SMTP_* environment variables.
func LoadSMTPConfig() *SMTPConfig {
	host := os.Getenv("ST_SMTP_HOST")
	if host == "" {
		return nil
	}
	return &SMTPConfig{
		Host:     host,
		Port:     cmp(os.Getenv("ST_SMTP_PORT"), "587"),
		User:     os.Getenv("ST_SMTP_USER"),
		Password: os.Getenv("ST_SMTP_PASSWORD"),
		From:     cmp(os.Getenv("ST_SMTP_FROM"), "syncthing@localhost"),
		UseTLS:   strings.ToLower(os.Getenv("ST_SMTP_TLS")) != "false",
	}
}

// IsConfigured returns true if SMTP is configured.
func (c *SMTPConfig) IsConfigured() bool {
	return c != nil && c.Host != ""
}

// Send sends a plain text email with the given subject and body.
func (c *SMTPConfig) Send(toEmail, subject, body string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("SMTP not configured")
	}
	return c.send(toEmail, subject, body)
}

// SendPasswordReset sends a password reset email with the given token.
func (c *SMTPConfig) SendPasswordReset(toEmail, username, resetToken, guiURL string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("SMTP not configured")
	}

	subject := "Syncthing Password Reset"
	body := fmt.Sprintf(
		"Hello %s,\r\n\r\n"+
			"A password reset has been requested for your Syncthing account.\r\n\r\n"+
			"Your reset token is:\r\n%s\r\n\r\n"+
			"Enter this token on the Syncthing login page to set a new password.\r\n"+
			"This token expires in 1 hour.\r\n\r\n"+
			"If you did not request this reset, you can safely ignore this email.\r\n",
		username, resetToken,
	)

	return c.send(toEmail, subject, body)
}

func (c *SMTPConfig) send(to, subject, body string) error {
	addr := net.JoinHostPort(c.Host, c.Port)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		c.From, to, subject, body,
	)

	var auth smtp.Auth
	if c.User != "" {
		auth = smtp.PlainAuth("", c.User, c.Password, c.Host)
	}

	if c.UseTLS {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: c.Host})
		if err != nil {
			return fmt.Errorf("TLS dial: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, c.Host)
		if err != nil {
			return fmt.Errorf("SMTP client: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP auth: %w", err)
			}
		}
		if err := client.Mail(c.From); err != nil {
			return fmt.Errorf("SMTP MAIL: %w", err)
		}
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("SMTP RCPT: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("SMTP DATA: %w", err)
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			return fmt.Errorf("SMTP write: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("SMTP close: %w", err)
		}
		return client.Quit()
	}

	if err := smtp.SendMail(addr, auth, c.From, []string{to}, []byte(msg)); err != nil {
		slog.Error("Failed to send email", "to", to, "error", err)
		return err
	}
	return nil
}

func cmp(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}
