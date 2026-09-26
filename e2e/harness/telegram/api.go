package telegram

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

const (
	Token       = "123456:test-token"
	BotUsername = "navidrome_tg_bot"
)

const AlwaysFail = -1

// API is a Telegram Bot API double running like a local server in
// --local mode: getFile answers with an absolute path of the file in its
// working directory. It records every call the bot makes.
type API struct {
	server  *httptest.Server
	workDir string

	mu       sync.Mutex
	calls    []*Call
	files    map[string]string
	failures map[string]int
	hold     chan struct{}
	held     chan struct{}
	sent     int
	uploaded []string
}

const UploadedFileParam = "uploaded_file"

func New(t *testing.T) *API {
	api := &API{
		workDir:  t.TempDir(),
		files:    map[string]string{},
		failures: map[string]int{},
	}
	api.server = httptest.NewServer(http.HandlerFunc(api.handle))
	t.Cleanup(api.server.Close)
	return api
}

func (a *API) URL() string {
	return a.server.URL
}

// AddFile puts a copy of the source into the server's working directory,
// as the server does when it downloads a file from Telegram.
func (a *API) AddFile(t *testing.T, fileID, source string) string {
	t.Helper()

	path := filepath.Join(a.workDir, "music", fileID+filepath.Ext(source))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755)) //nolint:gosec // G301: Navidrome container reads the library
	copyFile(t, source, path)

	a.mu.Lock()
	defer a.mu.Unlock()
	a.files[fileID] = path
	return path
}

func (a *API) FailGetFile(fileID string, n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failures[fileID] = n
}

// HoldGetFile makes getFile calls hang until ReleaseGetFile; the returned
// channel closes when the first one arrives.
func (a *API) HoldGetFile() <-chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hold = make(chan struct{})
	a.held = make(chan struct{})
	return a.held
}

func (a *API) ReleaseGetFile() {
	a.mu.Lock()
	defer a.mu.Unlock()
	close(a.hold)
	a.hold = nil
}

func (a *API) handle(w http.ResponseWriter, r *http.Request) {
	method, ok := strings.CutPrefix(r.URL.Path, "/bot"+Token+"/")
	if !ok {
		http.NotFound(w, r)
		return
	}

	params := map[string]string{}
	if r.ContentLength != 0 {
		if err := r.ParseMultipartForm(32 << 20); err != nil { //nolint:gosec // G120: fake Bot API serves only the test
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for key, values := range r.MultipartForm.Value {
			params[key] = values[0]
		}
		for _, files := range r.MultipartForm.File {
			params[UploadedFileParam] = files[0].Filename
		}
	}

	a.mu.Lock()
	call := &Call{Method: method, Params: params}
	a.calls = append(a.calls, call)
	a.mu.Unlock()

	switch method {
	case "getFile":
		if !a.waitHold(r) {
			return
		}
		a.getFile(w, params["file_id"])
	case "getMe":
		writeResult(w, map[string]any{"id": 123456, "is_bot": true, "first_name": "Navidrome", "username": BotUsername})
	case "sendMessage", "editMessageText":
		writeResult(w, a.botMessage(call, params))
	case "sendAudio":
		msg := a.botMessage(call, params)
		msg["audio"] = a.sentAudio(params)
		writeResult(w, msg)
	default:
		writeResult(w, true)
	}
}

// botMessage also notes the message id in the call.
func (a *API) botMessage(call *Call, params map[string]string) map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()

	id, err := strconv.Atoi(params["message_id"])
	if err != nil {
		a.sent++
		id = a.sent
	}
	call.MessageID = id
	chatID, _ := strconv.ParseInt(params["chat_id"], 10, 64)
	return map[string]any{"message_id": id, "date": 0, "chat": map[string]any{"id": chatID, "type": "private"}}
}

// sentAudio describes the audio of a sendAudio call: an uploaded file gets
// a new file_id, a file_id is sent as is.
func (a *API) sentAudio(params map[string]string) map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()

	id := params["audio"]
	if params[UploadedFileParam] != "" {
		id = fmt.Sprintf("uploaded-%d", len(a.uploaded)+1)
		a.uploaded = append(a.uploaded, id)
	}
	return map[string]any{"file_id": id, "file_unique_id": id + "-unique", "duration": 2}
}

func (a *API) UploadedFileID(n int) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n >= len(a.uploaded) {
		return ""
	}
	return a.uploaded[n]
}

func (a *API) waitHold(r *http.Request) bool {
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

func (a *API) getFile(w http.ResponseWriter, fileID string) {
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

func copyFile(t *testing.T, from, to string) {
	t.Helper()

	src, err := os.Open(from) //nolint:gosec // G304: paths come from the harness
	require.NoError(t, err)
	defer src.Close()

	dst, err := os.Create(to) //nolint:gosec // G304: paths come from the harness
	require.NoError(t, err)
	defer dst.Close()

	_, err = io.Copy(dst, src)
	require.NoError(t, err)
}

func (a *API) PathOf(fileID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.files[fileID]
}
