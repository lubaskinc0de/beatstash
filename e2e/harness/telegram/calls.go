package telegram

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

type Call struct {
	Method string
	Params map[string]string
	// MessageID is the message sent or edited.
	MessageID int
}

type InlineAnswer struct {
	QueryID    string
	Results    []InlineResult
	NextOffset string
	// Button goes above the results; its Text is empty without one.
	Button struct {
		Text           string `json:"text"`
		StartParameter string `json:"start_parameter"`
	}
}

type InlineResult struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     struct {
		Text string `json:"message_text"`
	} `json:"input_message_content"`
	AudioFileID    string `json:"audio_file_id"`
	DocumentFileID string `json:"document_file_id"`
	Caption        string `json:"caption"`
	ReplyMarkup    struct {
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

// InlineEdit is how the bot changed a message the user sent through
// inline mode.
type InlineEdit struct {
	Method string
	Text   string
	Media  struct {
		Type    string `json:"type"`
		Media   string `json:"media"`
		Caption string `json:"caption"`
	}
	Buttons []Button
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
	calls := make([]Call, 0, len(a.calls))
	for _, call := range a.calls {
		calls = append(calls, *call)
	}
	return calls
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
		answer := InlineAnswer{QueryID: call.Params["inline_query_id"], NextOffset: call.Params["next_offset"]}
		mustUnmarshal(t, call.Params["results"], &answer.Results)
		if button := call.Params["button"]; button != "" {
			mustUnmarshal(t, button, &answer.Button)
		}
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

func (a *API) InlineEdits(t *testing.T, inlineMessageID string) []InlineEdit {
	t.Helper()

	var edits []InlineEdit
	for _, call := range a.AllCalls() {
		if call.Params["inline_message_id"] != inlineMessageID {
			continue
		}
		edit := InlineEdit{Method: call.Method, Text: call.Params["text"]}
		if media := call.Params["media"]; media != "" {
			mustUnmarshal(t, media, &edit.Media)
		}
		edit.Buttons, _ = keyboardOf(call)
		edits = append(edits, edit)
	}
	return edits
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
	// SwitchInline is the query the button puts into the input field.
	SwitchInline string `json:"switch_inline_query_current_chat"`
}

// Buttons returns the inline keyboard the bot sent or set last.
func (a *API) Buttons(t *testing.T) []Button {
	t.Helper()

	_, buttons := a.lastWithKeyboard(func(Call, []Button) bool { return true })
	return buttons
}

// lastWithKeyboard returns the latest call with a keyboard that matches.
func (a *API) lastWithKeyboard(match func(Call, []Button) bool) (Call, []Button) {
	calls := a.AllCalls()
	for i := len(calls) - 1; i >= 0; i-- {
		if buttons, ok := keyboardOf(calls[i]); ok && match(calls[i], buttons) {
			return calls[i], buttons
		}
	}
	return Call{}, nil
}

func keyboardOf(call Call) ([]Button, bool) {
	markup := call.Params["reply_markup"]
	if markup == "" {
		return nil, false
	}
	var keyboard struct {
		Rows [][]Button `json:"inline_keyboard"`
	}
	if err := json.Unmarshal([]byte(markup), &keyboard); err != nil {
		panic(fmt.Sprintf("decode reply_markup %q: %v", markup, err))
	}
	var buttons []Button
	for _, row := range keyboard.Rows {
		buttons = append(buttons, row...)
	}
	return buttons, true
}

func (a *API) MessageWith(b Button) int {
	call, _ := a.lastWithKeyboard(func(_ Call, buttons []Button) bool { return slices.Contains(buttons, b) })
	return call.MessageID
}

// Window is the latest sent or edited message with a keyboard. Right after
// /share it is the share keyboard, not the window.
func (a *API) Window() Call {
	call, _ := a.lastWithKeyboard(func(call Call, _ []Button) bool {
		return call.Method == "sendMessage" || call.Method == "editMessageText"
	})
	return call
}

func (a *API) StrippedMessages() []int {
	var ids []int
	for _, call := range a.CallsTo("editMessageReplyMarkup") {
		if buttons, _ := keyboardOf(call); len(buttons) == 0 {
			id, _ := strconv.Atoi(call.Params["message_id"])
			ids = append(ids, id)
		}
	}
	return ids
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

func DocumentFileIDs(answer InlineAnswer) []string {
	var ids []string
	for _, result := range answer.Results {
		if result.DocumentFileID != "" {
			ids = append(ids, result.DocumentFileID)
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

// WaitCalls waits until the method has n calls or the time is up, and
// tells whether they came.
func (a *API) WaitCalls(method string, n int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if len(a.CallsTo(method)) >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return len(a.CallsTo(method)) >= n
}

// MessagesWithButtons lists the messages the bot sent with buttons and did
// not strip since.
func (a *API) MessagesWithButtons() []int {
	stripped := a.StrippedMessages()
	var ids []int
	for _, call := range a.CallsTo("sendMessage") {
		if buttons, _ := keyboardOf(call); len(buttons) > 0 && !slices.Contains(stripped, call.MessageID) {
			ids = append(ids, call.MessageID)
		}
	}
	return ids
}
