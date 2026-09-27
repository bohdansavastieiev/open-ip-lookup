package notify

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifySendsTelegramMessage(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		got = r.PostForm
	}))
	defer srv.Close()

	n := &Notifier{
		logger:  slog.New(slog.DiscardHandler),
		client:  srv.Client(),
		sendURL: srv.URL,
		chatID:  "42",
	}
	n.Notify(t.Context(), "source outdated")

	assert.Equal(t, "42", got.Get("chat_id"))
	assert.Equal(t, "source outdated", got.Get("text"))
}
