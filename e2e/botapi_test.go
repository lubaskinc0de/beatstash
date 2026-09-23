package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const botToken = "123456:test-token"

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

// botAPI is a Telegram Bot API double: it serves getFile and file downloads
// and records every call the bot makes.
type botAPI struct {
	server *httptest.Server

	mu    sync.Mutex
	calls []apiCall
	files map[string]string
}

func newBotAPI(t *testing.T) *botAPI {
	api := &botAPI{files: map[string]string{}}
	api.server = httptest.NewServer(http.HandlerFunc(api.handle))
	t.Cleanup(api.server.Close)
	return api
}

func (a *botAPI) URL() string {
	return a.server.URL
}

func (a *botAPI) addFile(fileID, localPath string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.files[fileID] = localPath
}

func (a *botAPI) handle(w http.ResponseWriter, r *http.Request) {
	if filePath, ok := strings.CutPrefix(r.URL.Path, "/file/bot"+botToken+"/"); ok {
		a.serveFile(w, r, filePath)
		return
	}

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
		a.getFile(w, params["file_id"])
	default:
		writeResult(w, true)
	}
}

func (a *botAPI) getFile(w http.ResponseWriter, fileID string) {
	a.mu.Lock()
	_, ok := a.files[fileID]
	a.mu.Unlock()

	if !ok {
		writeError(w, http.StatusBadRequest, "Bad Request: invalid file_id")
		return
	}

	writeResult(w, map[string]any{
		"file_id":        fileID,
		"file_unique_id": fileID + "-unique",
		"file_path":      "music/" + fileID,
	})
}

func (a *botAPI) serveFile(w http.ResponseWriter, r *http.Request, filePath string) {
	fileID, _ := strings.CutPrefix(filePath, "music/")

	a.mu.Lock()
	localPath, ok := a.files[fileID]
	a.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, localPath)
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

	var emojis []string
	for _, call := range a.callsTo("setMessageReaction") {
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
	return fmt.Sprintf("%s-%d", strings.ReplaceAll(name, ".", "-"), n)
}
