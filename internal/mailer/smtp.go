package mailer

import (
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
)

type Email struct {
	To      string
	Subject string
	Body    string
}

type Sender interface {
	Send(email Email) error
}

type Config struct {
	SMTPAddress  string
	SMTPUsername string
	SMTPPassword string
	From         string
}

type Client struct {
	config Config
}

func New(config Config) *Client {
	return &Client{config: config}
}

func (c *Client) Send(email Email) error {
	from, err := mail.ParseAddress(c.config.From)
	if err != nil {
		return fmt.Errorf("parse sender address: %w", err)
	}

	host, _, err := net.SplitHostPort(c.config.SMTPAddress)
	if err != nil {
		return fmt.Errorf("split SMTP address: %w", err)
	}

	var auth smtp.Auth

	if c.config.SMTPUsername != "" {
		auth = smtp.PlainAuth("", c.config.SMTPUsername, c.config.SMTPPassword, host)
	}

	message := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from.String(),
		email.To,
		email.Subject,
		email.Body,
	))

	if err := smtp.SendMail(c.config.SMTPAddress, auth, from.Address, []string{email.To}, message); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}
