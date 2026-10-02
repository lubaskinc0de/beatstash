package harness

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
)

func fileIDFor(name string, n int) string {
	return fmt.Sprintf("%s-%d", strings.ReplaceAll(filepath.Base(name), ".", "-"), n)
}

func AssertAlreadyExists(t *testing.T, s *Scenario, user User, messageID int) {
	t.Helper()

	replies := s.Telegram.Replies(t)
	require.Len(t, replies, 1)
	assert.Equal(t, messageID, replies[0].ReplyTo)
	assert.Contains(t, replies[0].Text, s.Catalog(user).AlreadyExists())
}

var inviteLink = regexp.MustCompile(`https://t\.me/` + telegram.BotUsername + `\?start=([A-Za-z0-9_-]+)`)

func (s *Scenario) Invite() string {
	s.t.Helper()

	s.Open(Admin, s.Catalog(Admin).AdminButton(), s.Catalog(Admin).InviteButton())
	return s.InviteCode()
}

func (s *Scenario) InviteCode() string {
	s.t.Helper()

	match := inviteLink.FindStringSubmatch(s.WindowText())
	require.NotNil(s.t, match, "no invite link in the window")
	return match[1]
}

func (s *Scenario) LastReply() telegram.Reply {
	s.t.Helper()

	replies := s.Telegram.Replies(s.t)
	require.NotEmpty(s.t, replies)
	return replies[len(replies)-1]
}

func (s *Scenario) nextUpdateID() int64 {
	return s.updates.Add(1)
}

// UploadAudio puts a fixture on the Bot API double, as if the user had
// uploaded it to Telegram, and returns the audio as Telegram describes it.
func (s *Scenario) UploadAudio(fixture string) models.Audio {
	s.t.Helper()

	return s.UploadAudioFile(audiofile.Fixture(fixture))
}

func (s *Scenario) UploadAudioFile(path string) models.Audio {
	s.t.Helper()

	fileID, size := s.Upload(path)
	return models.Audio{
		FileID:       fileID,
		FileUniqueID: fileID + "-unique",
		Duration:     1,
		FileName:     filepath.Base(path),
		FileSize:     size,
	}
}

func (s *Scenario) UploadDocument(path, mimeType string) models.Document {
	s.t.Helper()

	fileID, size := s.Upload(path)
	return models.Document{
		FileID:       fileID,
		FileUniqueID: fileID + "-unique",
		FileName:     filepath.Base(path),
		MimeType:     mimeType,
		FileSize:     size,
	}
}

func (s *Scenario) Upload(path string) (fileID string, size int64) {
	s.t.Helper()

	fileID = fileIDFor(path, int(s.uploads.Add(1)))
	stored := s.Telegram.AddFile(s.t, fileID, path)

	return fileID, FileSize(s.t, stored)
}

func (s *Scenario) AudioMessage(from User, audio models.Audio) *models.Update {
	return s.Message(from, func(m *models.Message) { m.Audio = &audio })
}

func (s *Scenario) DocumentMessage(from User, document models.Document) *models.Update {
	return s.Message(from, func(m *models.Message) { m.Document = &document })
}

func (s *Scenario) Message(from User, fill func(*models.Message)) *models.Update {
	msg := &models.Message{
		ID:   s.Telegram.NextMessageID(),
		From: from.telegram(),
		Chat: models.Chat{ID: from.ID, Type: models.ChatTypePrivate},
	}
	fill(msg)
	return &models.Update{ID: s.nextUpdateID(), Message: msg}
}

// join leaves the user Home, without a Navidrome Account.
func (s *Scenario) join(user User) {
	s.t.Helper()
	s.Join(user, s.Invite())
}

// Join lets the user in by the invite code and leaves them Home.
func (s *Scenario) Join(user User, code string) {
	s.t.Helper()
	s.Send(s.TextMessage(user, "/start "+code))
	s.Open(user)
}

func (s *Scenario) TextMessage(from User, text string) *models.Update {
	return s.Message(from, func(m *models.Message) { m.Text = text })
}

func (s *Scenario) InlineQuery(from User, query string) *models.Update {
	id := s.nextUpdateID()
	return &models.Update{
		ID: id,
		InlineQuery: &models.InlineQuery{
			ID:    fmt.Sprintf("inline-%d", id),
			From:  from.telegram(),
			Query: query,
		},
	}
}

// Search asks inline mode for the text and returns the answer; offset is
// the next_offset of the previous page.
func (s *Scenario) Search(from User, text, offset string) telegram.InlineAnswer {
	s.t.Helper()

	query := s.InlineQuery(from, text)
	query.InlineQuery.Offset = offset
	s.Send(query)
	return s.Telegram.InlineAnswerTo(s.t, query)
}

func (s *Scenario) Choose(from User, result telegram.InlineResult) *models.Update {
	update := s.ChosenResult(from, result)
	s.Send(update)
	return update
}

// ChosenResult is how Telegram reports the choice: with the id of the
// message it sent.
func (s *Scenario) ChosenResult(from User, result telegram.InlineResult) *models.Update {
	id := s.nextUpdateID()
	return &models.Update{
		ID: id,
		ChosenInlineResult: &models.ChosenInlineResult{
			ResultID:        result.ID,
			From:            *from.telegram(),
			InlineMessageID: fmt.Sprintf("chosen-message-%d", id),
		},
	}
}

// Press taps the button on the latest message that has it.
func (s *Scenario) Press(from User, b telegram.Button) *models.Update {
	return s.PressOn(from, b, s.Telegram.MessageWith(b))
}

// PressOn taps the button on the message, as if it still had the button.
func (s *Scenario) PressOn(from User, b telegram.Button, messageID int) *models.Update {
	update := s.CallbackQuery(from, b.Data)
	update.CallbackQuery.Message = models.MaybeInaccessibleMessage{
		Type:    models.MaybeInaccessibleMessageTypeMessage,
		Message: &models.Message{ID: messageID, Chat: models.Chat{ID: from.ID, Type: models.ChatTypePrivate}},
	}
	s.Send(update)
	return update
}

func (s *Scenario) CallbackQuery(from User, data string) *models.Update {
	id := s.nextUpdateID()
	return &models.Update{
		ID: id,
		CallbackQuery: &models.CallbackQuery{
			ID:   fmt.Sprintf("callback-%d", id),
			From: *from.telegram(),
			Data: data,
		},
	}
}

func (s *Scenario) NowPlaying(user User) telegram.InlineResult {
	s.t.Helper()

	query := s.InlineQuery(user, "np")
	s.Send(query)
	results := s.Telegram.InlineAnswerTo(s.t, query).Results
	require.Len(s.t, results, 1)
	return results[0]
}

// Uploaded sends the audio as the user and waits until it is ingested.
func (s *Scenario) Uploaded(from User, audio models.Audio) *models.Update {
	s.t.Helper()

	upload := s.AudioMessage(from, audio)
	s.Send(upload)
	s.WaitIngest()
	return upload
}

func (s *Scenario) UploadAlbum(from User, album string, tracks int) []*models.Update {
	s.t.Helper()

	uploads := make([]*models.Update, 0, tracks)
	for n := 1; n <= tracks; n++ {
		tags := map[string]string{
			"artist": "Artist", "album": album, "track": fmt.Sprint(n), "title": fmt.Sprintf("Song %d", n),
		}
		file := audiofile.Generate(s.t, fmt.Sprintf("song%d.mp3", n), audiofile.Spec{Tags: tags})
		uploads = append(uploads, s.Uploaded(from, s.UploadAudioFile(file)))
	}
	return uploads
}

func (s *Scenario) ReplyCommand(from User, command string, to *models.Update) *models.Update {
	return s.Message(from, func(m *models.Message) {
		m.Text = command
		m.ReplyToMessage = to.Message
	})
}

func (s *Scenario) Button(name string) telegram.Button {
	s.t.Helper()
	return telegram.ButtonNamed(s.t, s.Telegram.Buttons(s.t), name)
}

func (s *Scenario) LastCallbackAnswer() string {
	s.t.Helper()

	answers := s.Telegram.CallbackAnswers()
	require.NotEmpty(s.t, answers)
	return answers[len(answers)-1]
}

// BotAudio is the audio the bot sent to the user, as the user's client shows it.
func (s *Scenario) BotAudio(to User, audio models.Audio) *models.Update {
	update := s.AudioMessage(to, audio)
	update.Message.From = &models.User{ID: 123456, IsBot: true, Username: telegram.BotUsername}
	return update
}

func (s *Scenario) SentMessagesContaining(to User, text string) []string {
	chatID := strconv.FormatInt(to.ID, 10)
	var texts []string
	for _, call := range s.Telegram.CallsTo("sendMessage") {
		if call.Params["chat_id"] == chatID && strings.Contains(call.Params["text"], text) {
			texts = append(texts, call.Params["text"])
		}
	}
	return texts
}

func (s *Scenario) WindowText() string {
	s.t.Helper()

	window := s.Telegram.Window()
	require.NotEmpty(s.t, window.Method, "the bot showed no window")
	return window.Params["text"]
}

func (s *Scenario) Open(user User, buttons ...string) {
	s.t.Helper()

	s.Send(s.TextMessage(user, "/start"))
	s.Go(user, buttons...)
}

func (s *Scenario) Go(user User, buttons ...string) {
	s.t.Helper()

	for _, name := range buttons {
		s.Press(user, s.Button(name))
	}
}
