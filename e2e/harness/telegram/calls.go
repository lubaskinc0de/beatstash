package telegram

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"

	"github.com/go-telegram/bot/models"
)

type Call struct {
	Method string
	Params map[string]string
}

type InlineAnswer struct {
	QueryID string
	Results []InlineResult
}

type InlineResult struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	AudioFileID string `json:"audio_file_id"`
	Caption     string `json:"caption"`
	ReplyMarkup struct {
		Rows [][]Button `json:"inline_keyboard"`
	} `json:"reply_markup"`
}

func (r InlineResult) Buttons() []Button {
	var buttons []Button
	for _, row := range r.ReplyMarkup.Rows {
		buttons = append(buttons, row...)
	}
	return buttons
}

type Reply struct {
	ReplyTo int
	Text    string
}

// Forget drops the calls recorded so far, so a scenario sees only its own.
func (a *API) Forget() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = nil
}

func (a *API) AllCalls() []Call {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Call(nil), a.calls...)
}

func (a *API) CallsTo(method string) []Call {
	var result []Call
	for _, call := range a.AllCalls() {
		if call.Method == method {
			result = append(result, call)
		}
	}
	return result
}

// Reactions returns the emojis the bot set on messages, in order,
// skipping calls that only clear the reaction.
func (a *API) Reactions(t *testing.T) []string {
	t.Helper()
	return a.reactionsWhere(t, func(Call) bool { return true })
}

func (a *API) ReactionsOn(t *testing.T, messageID int) []string {
	t.Helper()
	return a.reactionsWhere(t, func(call Call) bool {
		return call.Params["message_id"] == strconv.Itoa(messageID)
	})
}

func (a *API) reactionsWhere(t *testing.T, match func(Call) bool) []string {
	t.Helper()

	var emojis []string
	for _, call := range a.CallsTo("setMessageReaction") {
		if !match(call) {
			continue
		}
		var reactions []struct {
			Emoji string `json:"emoji"`
		}
		mustUnmarshal(t, call.Params["reaction"], &reactions)
		for _, reaction := range reactions {
			emojis = append(emojis, reaction.Emoji)
		}
	}
	return emojis
}

func (a *API) Replies(t *testing.T) []Reply {
	t.Helper()

	var replies []Reply
	for _, call := range a.CallsTo("sendMessage") {
		r := Reply{Text: call.Params["text"]}
		if params := call.Params["reply_parameters"]; params != "" {
			var p struct {
				MessageID int `json:"message_id"`
			}
			mustUnmarshal(t, params, &p)
			r.ReplyTo = p.MessageID
		}
		replies = append(replies, r)
	}
	return replies
}

func (a *API) InlineAnswers(t *testing.T) []InlineAnswer {
	t.Helper()

	var answers []InlineAnswer
	for _, call := range a.CallsTo("answerInlineQuery") {
		answer := InlineAnswer{QueryID: call.Params["inline_query_id"]}
		mustUnmarshal(t, call.Params["results"], &answer.Results)
		answers = append(answers, answer)
	}
	return answers
}

func (a *API) InlineAnswerTo(t *testing.T, query *models.Update) InlineAnswer {
	t.Helper()

	for _, answer := range a.InlineAnswers(t) {
		if answer.QueryID == query.InlineQuery.ID {
			return answer
		}
	}
	t.Fatalf("inline query %s is not answered", query.InlineQuery.ID)
	return InlineAnswer{}
}

func (a *API) DeletedMessages() []string {
	var ids []string
	for _, call := range a.CallsTo("deleteMessage") {
		ids = append(ids, call.Params["message_id"])
	}
	return ids
}

// AnsweredCallbacks returns ids of the callback queries the bot answered.
func (a *API) AnsweredCallbacks() []string {
	var ids []string
	for _, call := range a.CallsTo("answerCallbackQuery") {
		ids = append(ids, call.Params["callback_query_id"])
	}
	return ids
}

type Button struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

// Buttons returns the inline keyboard the bot sent or set last.
func (a *API) Buttons(t *testing.T) []Button {
	t.Helper()

	calls := a.AllCalls()
	for i := len(calls) - 1; i >= 0; i-- {
		markup := calls[i].Params["reply_markup"]
		if markup == "" {
			continue
		}
		var keyboard struct {
			Rows [][]Button `json:"inline_keyboard"`
		}
		mustUnmarshal(t, markup, &keyboard)
		var buttons []Button
		for _, row := range keyboard.Rows {
			buttons = append(buttons, row...)
		}
		return buttons
	}
	return nil
}

// CallbackAnswers returns the texts the bot showed on button presses.
func (a *API) CallbackAnswers() []string {
	var texts []string
	for _, call := range a.CallsTo("answerCallbackQuery") {
		texts = append(texts, call.Params["text"])
	}
	return texts
}

func mustUnmarshal(t *testing.T, data string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(data), v); err != nil {
		t.Fatalf("decode bot api param %q: %v", data, err)
	}
}

func AudioFileIDs(answer InlineAnswer) []string {
	var ids []string
	for _, result := range answer.Results {
		if result.AudioFileID != "" {
			ids = append(ids, result.AudioFileID)
		}
	}
	return ids
}

func ButtonNamed(t *testing.T, buttons []Button, name string) Button {
	t.Helper()

	for _, b := range buttons {
		if b.Text == name {
			return b
		}
	}
	t.Fatalf("no button %q among %v", name, buttons)
	return Button{}
}

func ButtonTexts(buttons []Button) []string {
	var texts []string
	for _, b := range buttons {
		texts = append(texts, b.Text)
	}
	return texts
}

func (a *API) EditedMessages() []string {
	var ids []string
	for _, call := range a.CallsTo("editMessageText") {
		id := call.Params["message_id"]
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}
