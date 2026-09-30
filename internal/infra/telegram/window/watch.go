package window

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// CountArrivals counts the user's messages below the window.
func CountArrivals(windows *Windows) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			if msg := update.Message; msg != nil {
				if err := windows.Arrived(ctx, msg.Chat.ID, msg.ID); err != nil {
					slog.Error("count_message", "error", err)
				}
			}
			next(ctx, b, update)
		}
	}
}

// Watch counts the bot's messages below the window: it sees every call, so
// no send, the Poller's included, slips by.
func Watch(windows *Windows, next bot.HttpClient) bot.HttpClient {
	return &watchingClient{next: next, windows: windows}
}

type watchingClient struct {
	next    bot.HttpClient
	windows *Windows
}

func (c *watchingClient) Do(req *http.Request) (*http.Response, error) {
	method := path.Base(req.URL.Path)
	switch {
	case method == "deleteMessage":
		gone := deletedMessage(req)
		resp, result, err := c.do(req)
		if result != nil {
			c.note(c.windows.Deleted(req.Context(), gone.chatID, gone.messageID))
		}
		return resp, err
	case strings.HasPrefix(method, "send"):
		resp, result, err := c.do(req)
		for _, sent := range sentMessages(result) {
			c.note(c.windows.Arrived(req.Context(), sent.Chat.ID, sent.MessageID))
		}
		return resp, err
	default:
		return c.next.Do(req)
	}
}

func (c *watchingClient) note(err error) {
	if err != nil {
		slog.Error("count_message", "error", err)
	}
}

// do also returns the result of a successful call; the response keeps its
// body for the bot library.
func (c *watchingClient) do(req *http.Request) (*http.Response, json.RawMessage, error) {
	resp, err := c.next.Do(req)
	if err != nil {
		return resp, nil, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return resp, nil, err
	}
	return resp, succeeded(body), nil
}

// succeeded returns nil for an answer of a failed call.
func succeeded(body []byte) json.RawMessage {
	var answer struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(body, &answer) != nil || !answer.OK {
		return nil
	}
	return answer.Result
}

type sentMessage struct {
	MessageID int `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

// sentMessages reads a message or, for an album, a list; other results,
// like of sendChatAction, give none.
func sentMessages(result json.RawMessage) []sentMessage {
	var many []sentMessage
	if json.Unmarshal(result, &many) != nil {
		var one sentMessage
		if json.Unmarshal(result, &one) != nil {
			return nil
		}
		many = []sentMessage{one}
	}
	sent := many[:0]
	for _, msg := range many {
		if msg.MessageID != 0 {
			sent = append(sent, msg)
		}
	}
	return sent
}

// deletedMessage reads the multipart form the bot library sends; the
// request keeps its body.
func deletedMessage(req *http.Request) message {
	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || req.GetBody == nil {
		return message{}
	}
	body, err := req.GetBody()
	if err != nil {
		return message{}
	}
	form, err := multipart.NewReader(body, params["boundary"]).ReadForm(1 << 10)
	if err != nil {
		slog.Error("read_deleted_message", "error", err)
		return message{}
	}
	defer func() { _ = form.RemoveAll() }()
	chatID, _ := strconv.ParseInt(formValue(form, "chat_id"), 10, 64)
	messageID, _ := strconv.Atoi(formValue(form, "message_id"))
	return message{chatID: chatID, messageID: messageID}
}

type message struct {
	chatID    int64
	messageID int
}

func formValue(form *multipart.Form, key string) string {
	if values := form.Value[key]; len(values) > 0 {
		return values[0]
	}
	return ""
}
