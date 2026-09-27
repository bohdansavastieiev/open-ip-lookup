package notify

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func telegramServer(t *testing.T) (*httptest.Server, *[]url.Values) {
	t.Helper()
	var got []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		got = append(got, r.PostForm)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestNotifySendsTelegramMessage(t *testing.T) {
	srv, got := telegramServer(t)
	n := newNotifier(slog.New(slog.DiscardHandler), srv.URL, "42")

	n.Notify("source outdated")
	n.Close()

	require.Len(t, *got, 1)
	assert.Equal(t, "42", (*got)[0].Get("chat_id"))
	assert.Equal(t, "source outdated", (*got)[0].Get("text"))
}

func TestHandlerSendsOnlyErrors(t *testing.T) {
	srv, got := telegramServer(t)
	n := newNotifier(slog.New(slog.DiscardHandler), srv.URL, "42")
	logger := slog.New(NewHandler(slog.NewTextHandler(io.Discard, nil), n))

	logger.Warn("source refresh failed")
	logger.Error("create share", slog.String("err", "database is locked"))
	n.Close()

	require.Len(t, *got, 1)
	assert.Equal(t, "Error: create share\nerr=database is locked", (*got)[0].Get("text"))
}
