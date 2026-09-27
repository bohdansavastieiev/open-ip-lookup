// Package notify delivers operator alerts: always to logs, and to Telegram when configured.
package notify

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	telegramBotTokenEnv = "TELEGRAM_BOT_TOKEN"
	telegramChatIDEnv   = "TELEGRAM_CHAT_ID"
	telegramAPIURL      = "https://api.telegram.org"
	sendTimeout         = 30 * time.Second
	queueSize           = 100
	closeTimeout        = 10 * time.Second

	// Telegram rejects messages over 4096 characters.
	telegramMaxMessageLen = 4000
)

var (
	errBuildTelegramRequest = errors.New("build telegram request")
	errSendTelegramRequest  = errors.New("send telegram request")
)

// Notifier sends messages from a background worker, so a slow Telegram never blocks the caller.
type Notifier struct {
	logger  *slog.Logger
	client  *http.Client
	sendURL string
	chatID  string
	queue   chan string
	pending sync.WaitGroup
}

// New reads Telegram credentials from the environment. Without them alerts are only logged.
func New(logger *slog.Logger) *Notifier {
	chatID := os.Getenv(telegramChatIDEnv)
	token := os.Getenv(telegramBotTokenEnv)
	var sendURL string
	if token != "" && chatID != "" {
		sendURL = telegramAPIURL + "/bot" + token + "/sendMessage"
	}
	logger.Info("notifier configured", slog.Bool("telegram", sendURL != ""))
	return newNotifier(logger, sendURL, chatID)
}

func newNotifier(logger *slog.Logger, sendURL, chatID string) *Notifier {
	n := &Notifier{
		logger:  logger,
		client:  &http.Client{Timeout: sendTimeout},
		sendURL: sendURL,
		chatID:  chatID,
		queue:   make(chan string, queueSize),
	}
	if sendURL != "" {
		go n.run()
	}
	return n
}

// Notify logs an alert and sends it to Telegram.
func (n *Notifier) Notify(message string) {
	n.logger.Warn("alert", slog.String("message", message))
	n.send(message)
}

// Close waits for queued messages to be sent, at most closeTimeout.
func (n *Notifier) Close() {
	done := make(chan struct{})
	go func() {
		n.pending.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(closeTimeout):
	}
}

func (n *Notifier) send(message string) {
	if n.sendURL == "" {
		return
	}
	n.pending.Add(1)
	select {
	case n.queue <- message:
	default:
		n.pending.Done()
		n.logger.Warn("telegram message dropped, queue is full")
	}
}

func (n *Notifier) run() {
	for message := range n.queue {
		if err := n.sendTelegram(message); err != nil {
			n.logger.Warn("send telegram message failed", slog.Any("err", err))
		}
		n.pending.Done()
	}
}

// Errors from building or sending the request embed the URL, which holds the bot token,
// so they are replaced instead of wrapped.
func (n *Notifier) sendTelegram(message string) error {
	if len(message) > telegramMaxMessageLen {
		message = message[:telegramMaxMessageLen]
	}
	form := url.Values{"chat_id": {n.chatID}, "text": {message}}
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequest(http.MethodPost, n.sendURL, body)
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
