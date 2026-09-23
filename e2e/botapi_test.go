package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const botToken = "123456:test-token"

const alwaysFail = -1

type apiCall struct {
	Method string
	Params map[string]string
}

type inlineAnswer struct {
	QueryID string
	Results []inlineResult
}

type inlineResult struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	AudioFileID string `json:"audio_file_id"`
	Caption     string `json:"caption"`
}

type reply struct {
	ReplyTo int
	Text    string
}

// botAPI is a Telegram Bot API double running like a local server in
// --local mode: getFile answers with an absolute path of the file in its
// working directory. It records every call the bot makes.
type botAPI struct {
	server  *httptest.Server
	workDir string

	mu       sync.Mutex
	calls    []apiCall
	files    map[string]string
	failures map[string]int
	hold     chan struct{}
	held     chan struct{}
}

func newBotAPI(t *testing.T) *botAPI {
	api := &botAPI{
		workDir:  t.TempDir(),
		files:    map[string]string{},
		failures: map[string]int{},
	}
	api.server = httptest.NewServer(http.HandlerFunc(api.handle))
	t.Cleanup(api.server.Close)
	return api
}

func (a *botAPI) URL() string {
	return a.server.URL
}

// addFile puts a copy of the source into the server's working directory,
// as the server does when it downloads a file from Telegram.
func (a *botAPI) addFile(t *testing.T, fileID, source string) string {
	t.Helper()

	path := filepath.Join(a.workDir, "music", fileID+filepath.Ext(source))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	copyFile(t, source, path)

	a.mu.Lock()
	defer a.mu.Unlock()
	a.files[fileID] = path
	return path
}

func (a *botAPI) failGetFile(fileID string, n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failures[fileID] = n
}

// holdGetFile makes getFile calls hang until releaseGetFile; the returned
// channel closes when the first one arrives.
func (a *botAPI) holdGetFile() <-chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hold = make(chan struct{})
	a.held = make(chan struct{})
	return a.held
}

func (a *botAPI) releaseGetFile() {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.hold)
	a.hold = nil
}

func (a *botAPI) handle(w http.ResponseWriter, r *http.Request) {
	method, ok := strings.CutPrefix(r.URL.Path, "/bot"+botToken+"/")
	if !ok {
		http.NotFound(w, r)
		return
	}

	params := map[string]string{}
	if r.ContentLength != 0 {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for key, values := range r.MultipartForm.Value {
			params[key] = values[0]
		}
	}

	a.mu.Lock()
	a.calls = append(a.calls, apiCall{Method: method, Params: params})
	a.mu.Unlock()

	switch method {
	case "getFile":
		if !a.waitHold(r) {
			return
		}
		a.getFile(w, params["file_id"])
	case "sendMessage":
		writeResult(w, map[string]any{"message_id": 1, "date": 0, "chat": map[string]any{"id": 1, "type": "private"}})
	default:
		writeResult(w, true)
	}
}

func (a *botAPI) waitHold(r *http.Request) bool {
	a.mu.Lock()
	hold, held := a.hold, a.held
	if hold != nil {
		select {
		case <-held:
		default:
			close(held)
		}
	}
	a.mu.Unlock()

	if hold == nil {
		return true
	}
	select {
	case <-hold:
		return true
	case <-r.Context().Done():
		return false
	}
}

func (a *botAPI) getFile(w http.ResponseWriter, fileID string) {
	a.mu.Lock()
	path, ok := a.files[fileID]
	failures := a.failures[fileID]
	if failures > 0 {
		a.failures[fileID]--
	}
	a.mu.Unlock()

	if failures != 0 {
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	if !ok {
		writeError(w, http.StatusBadRequest, "Bad Request: invalid file_id")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Bad Request: file is gone")
		return
	}

	writeResult(w, map[string]any{
		"file_id":        fileID,
		"file_unique_id": fileID + "-unique",
		"file_size":      info.Size(),
		"file_path":      path,
	})
}

func writeResult(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}

func writeError(w http.ResponseWriter, code int, description string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":          false,
		"error_code":  code,
		"description": description,
	})
}

func (a *botAPI) Calls() []apiCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]apiCall(nil), a.calls...)
}

func (a *botAPI) callsTo(method string) []apiCall {
	var result []apiCall
	for _, call := range a.Calls() {
		if call.Method == method {
			result = append(result, call)
		}
	}
	return result
}

// Reactions returns the emojis the bot set on messages, in order,
// skipping calls that only clear the reaction.
func (a *botAPI) Reactions(t *testing.T) []string {
	t.Helper()
	return a.reactions(t, func(apiCall) bool { return true })
}

func (a *botAPI) ReactionsOn(t *testing.T, messageID int) []string {
	t.Helper()
	return a.reactions(t, func(call apiCall) bool {
		return call.Params["message_id"] == strconv.Itoa(messageID)
	})
}

func (a *botAPI) reactions(t *testing.T, match func(apiCall) bool) []string {
	t.Helper()

	var emojis []string
	for _, call := range a.callsTo("setMessageReaction") {
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

func (a *botAPI) Replies(t *testing.T) []reply {
	t.Helper()

	var replies []reply
	for _, call := range a.callsTo("sendMessage") {
		r := reply{Text: call.Params["text"]}
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

func (a *botAPI) InlineAnswers(t *testing.T) []inlineAnswer {
	t.Helper()

	var answers []inlineAnswer
	for _, call := range a.callsTo("answerInlineQuery") {
		answer := inlineAnswer{QueryID: call.Params["inline_query_id"]}
		mustUnmarshal(t, call.Params["results"], &answer.Results)
		answers = append(answers, answer)
	}
	return answers
}

// AnsweredCallbacks returns ids of the callback queries the bot answered.
func (a *botAPI) AnsweredCallbacks() []string {
	var ids []string
	for _, call := range a.callsTo("answerCallbackQuery") {
		ids = append(ids, call.Params["callback_query_id"])
	}
	return ids
}

func mustUnmarshal(t *testing.T, data string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(data), v); err != nil {
		t.Fatalf("decode bot api param %q: %v", data, err)
	}
}

func fileIDFor(name string, n int) string {
	return fmt.Sprintf("%s-%d", strings.ReplaceAll(filepath.Base(name), ".", "-"), n)
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()

	src, err := os.Open(from)
	require.NoError(t, err)
	defer src.Close()

	dst, err := os.Create(to)
	require.NoError(t, err)
	defer dst.Close()

	_, err = io.Copy(dst, src)
	require.NoError(t, err)
}

func (a *botAPI) pathOf(fileID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.files[fileID]
}
