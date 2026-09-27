package notify

import (
	"context"
	"log/slog"
	"strings"
)

// Handler passes every record to the wrapped handler and also sends error records to Telegram.
type Handler struct {
	slog.Handler
	notifier *Notifier
}

func NewHandler(handler slog.Handler, notifier *Notifier) *Handler {
	return &Handler{Handler: handler, notifier: notifier}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		h.notifier.send(errorMessage(r))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewHandler(h.Handler.WithAttrs(attrs), h.notifier)
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return NewHandler(h.Handler.WithGroup(name), h.notifier)
}

func errorMessage(r slog.Record) string {
	var b strings.Builder
	b.WriteString("Error: ")
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString("\n")
		b.WriteString(a.String())
		return true
	})
	return b.String()
}
