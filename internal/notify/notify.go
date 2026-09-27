// Package notify delivers operator alerts: always to logs, and to Telegram when configured.
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	telegramBotTokenEnv = "TELEGRAM_BOT_TOKEN"
	telegramChatIDEnv   = "TELEGRAM_CHAT_ID"
	telegramAPIURL      = "https://api.telegram.org"
	sendTimeout         = 30 * time.Second

	// Telegram rejects messages over 4096 characters.
	telegramMaxMessageLen = 4000
)

var (
	errBuildTelegramRequest = errors.New("build telegram request")
	errSendTelegramRequest  = errors.New("send telegram request")
)

type Notifier struct {
	logger  *slog.Logger
	client  *http.Client
	sendURL string
	chatID  string
}

// New reads Telegram credentials from the environment. Without them alerts are only logged.
func New(logger *slog.Logger) *Notifier {
	n := &Notifier{
		logger: logger,
		client: &http.Client{Timeout: sendTimeout},
		chatID: os.Getenv(telegramChatIDEnv),
	}
	token := os.Getenv(telegramBotTokenEnv)
	if token != "" && n.chatID != "" {
		n.sendURL = telegramAPIURL + "/bot" + token + "/sendMessage"
	}
	logger.Info("notifier configured", slog.Bool("telegram", n.sendURL != ""))
	return n
}

func (n *Notifier) Notify(ctx context.Context, message string) {
	n.logger.Warn("alert", slog.String("message", message))
	if n.sendURL == "" {
		return
	}
	if err := n.sendTelegram(ctx, message); err != nil {
		n.logger.Warn("send telegram alert failed", slog.Any("err", err))
	}
}

// Errors from building or sending the request embed the URL, which holds the bot token,
// so they are replaced instead of wrapped.
func (n *Notifier) sendTelegram(ctx context.Context, message string) error {
	if len(message) > telegramMaxMessageLen {
		message = message[:telegramMaxMessageLen]
	}
	form := url.Values{"chat_id": {n.chatID}, "text": {message}}
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.sendURL, body)
	if err != nil {
		return errBuildTelegramRequest
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := n.client.Do(req)
	if err != nil {
		return errSendTelegramRequest
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram status %d", resp.StatusCode)
	}
	return nil
}
