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
	holds    map[string]*Hold
	messages int
	uploaded []string
	refusals int
	outages  map[string]outage
}

type outage struct {
	calls int
	code  int
}

const UploadedFileParam = "uploaded_file"

func New(t *testing.T) *API {
	api := &API{
		workDir:  t.TempDir(),
		files:    map[string]string{},
		failures: map[string]int{},
		holds:    map[string]*Hold{},
		outages:  map[string]outage{},
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

// RefuseUploads makes Telegram turn down the next n uploaded files.
func (a *API) RefuseUploads(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.refusals = n
}

func (a *API) refuseUpload(params map[string]string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if params[UploadedFileParam] == "" || a.refusals == 0 {
		return false
	}
	a.refusals--
	return true
}

// FailCalls makes the next n calls to the method fail with the HTTP code.
// A failed call is not noted: the user never sees it.
func (a *API) FailCalls(method string, n, code int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.outages[method] = outage{calls: n, code: code}
}

func (a *API) failCall(method string) (code int, failed bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	o := a.outages[method]
	if o.calls == 0 {
		return 0, false
	}
	o.calls--
	a.outages[method] = o
	return o.code, true
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

	if !a.waitCallHold(r, method) {
		return
	}
	if code, failed := a.failCall(method); failed {
		writeError(w, code, http.StatusText(code))
		return
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
		if a.refuseUpload(params) {
			writeError(w, http.StatusBadRequest, "Bad Request: wrong file")
			return
		}
		msg := a.botMessage(call, params)
		msg["audio"] = a.sentAudio(params)
		writeResult(w, msg)
	default:
		writeResult(w, true)
	}
}

func (a *API) botMessage(call *Call, params map[string]string) map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()

	id, err := strconv.Atoi(params["message_id"])
	if err != nil {
		id = a.nextMessageID()
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

// NextMessageID numbers the messages of users and of the bot in one
// sequence, as Telegram does within a chat.
func (a *API) NextMessageID() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.nextMessageID()
}

func (a *API) nextMessageID() int {
	a.messages++
	return a.messages
}

func (a *API) UploadedFileID(n int) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n >= len(a.uploaded) {
		return ""
	}
	return a.uploaded[n]
}

// Hold keeps one call to a method from Telegram until Release.
type Hold struct {
	arrived chan struct{}
	release chan struct{}
}

// Arrived closes when the held call comes.
func (h *Hold) Arrived() <-chan struct{} {
	return h.arrived
}

func (h *Hold) Release() {
	close(h.release)
}

// Hold makes the next call to the method hang until Release: the call
// reaches Telegram only then, and never if the bot gives up on it first.
func (a *API) Hold(method string) *Hold {
	a.mu.Lock()
	defer a.mu.Unlock()
	h := &Hold{arrived: make(chan struct{}), release: make(chan struct{})}
	a.holds[method] = h
	return h
}

func (a *API) waitCallHold(r *http.Request, method string) bool {
	a.mu.Lock()
	h := a.holds[method]
	delete(a.holds, method)
	a.mu.Unlock()
	if h == nil {
		return true
	}
	close(h.arrived)
	select {
	case <-h.release:
		return true
	case <-r.Context().Done():
		return false
	}
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
