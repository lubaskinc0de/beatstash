package e2e

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/internal/app"
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
)

const fixtureTitle = "Fixture Song"

type telegramUser struct {
	ID       int64
	Username string
}

const secretKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

var (
	adminUser = telegramUser{ID: 1000, Username: "boss"}
	alice     = telegramUser{ID: 1001, Username: "alice"}
	bob       = telegramUser{ID: 1002, Username: "bob"}
	stranger  = telegramUser{ID: 6666, Username: "mallory"}
)

var databaseSeq atomic.Int64

// newcomer is a Telegram user unknown to the bot, with a username free in Navidrome.
func newcomer(name string) telegramUser {
	return telegramUser{ID: 2000 + navidromeLoginSeq.Add(1), Username: uniqueLogin(name)}
}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type scenario struct {
	t       *testing.T
	clock   *clock
	config  config.Config
	app     *app.App
	stop    func()
	botAPI  *botAPI
	library string

	updates  int64
	messages int
	uploads  int
}

type scenarioOption func(*config.Config)

func withAdminContact(contact string) scenarioOption {
	return func(c *config.Config) { c.AdminContact = contact }
}

func withoutWorkers() scenarioOption {
	return func(c *config.Config) { c.IngestWorkers = 0 }
}

func newScenario(t *testing.T, opts ...scenarioOption) *scenario {
	t.Helper()

	s := prepareScenario(t, opts...)
	s.start()
	s.join(alice)
	s.join(bob)
	s.botAPI.forget()
	return s
}

// prepareScenario sets up everything a bot needs but does not start it.
func prepareScenario(t *testing.T, opts ...scenarioOption) *scenario {
	t.Helper()

	api := newBotAPI(t)

	library, err := os.MkdirTemp(env.libraryRoot, "scenario-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(library, 0o755))

	clk := &clock{now: time.Now()}
	cfg := config.Config{
		Clock:              clk.Now,
		InviteTTL:          7 * 24 * time.Hour,
		Token:              botToken,
		BotApiUrl:          api.URL(),
		DbDsn:              postgresDSN(createDatabase(t)),
		MusicDir:           library,
		NavidromeMusicDir:  navidromeLibraryMount + "/" + filepath.Base(library),
		AdminIds:           []uint64{uint64(adminUser.ID)},
		SecretKey:          secretKey,
		NavidromeUser:      navidromeAdmin,
		NavidromePassword:  navidromePassword,
		NavidromeUrl:       env.navidrome.url,
		IngestWorkers:      2,
		IngestRetryDelays:  []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond},
		IngestPollInterval: 10 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &scenario{t: t, clock: clk, config: cfg, botAPI: api, library: library}
}

func (s *scenario) start() {
	s.t.Helper()

	a, err := app.New(s.config, bot.WithSkipGetMe(), bot.WithNotAsyncHandlers())
	require.NoError(s.t, err)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := a.StartWorkers(ctx)
	var once sync.Once
	s.app = a
	s.stop = func() {
		once.Do(func() {
			cancel()
			<-stopped
			_ = a.Close()
		})
	}
	s.t.Cleanup(s.stop)
}

// restart stops the bot, even mid-Ingest, and starts it again on the same
// database and Library, this time with Ingest workers.
func (s *scenario) restart(opts ...scenarioOption) {
	s.t.Helper()

	s.stop()
	s.config.IngestWorkers = 2
	for _, opt := range opts {
		opt(&s.config)
	}
	s.start()
}

func createDatabase(t *testing.T) string {
	t.Helper()

	name := fmt.Sprintf("scenario_%d", databaseSeq.Add(1))

	conn, err := pgx.Connect(t.Context(), postgresDSN("postgres"))
	require.NoError(t, err)
	defer conn.Close(context.Background())

	_, err = conn.Exec(t.Context(), "CREATE DATABASE "+name)
	require.NoError(t, err)
	return name
}

func (s *scenario) send(update *models.Update) {
	s.app.Bot().ProcessUpdate(s.t.Context(), update)
}

func (s *scenario) waitIngest() {
	s.t.Helper()

	ctx, cancel := context.WithTimeout(s.t.Context(), time.Minute)
	defer cancel()
	require.NoError(s.t, s.app.WaitIngest(ctx))
}

func (s *scenario) nextUpdateID() int64 {
	s.updates++
	return s.updates
}

// uploadAudio puts a fixture on the Bot API double, as if the user had
// uploaded it to Telegram, and returns the audio as Telegram describes it.
func (s *scenario) uploadAudio(fixture string) models.Audio {
	s.t.Helper()

	return s.uploadAudioFile(fixturePath(fixture))
}

func (s *scenario) uploadAudioFile(path string) models.Audio {
	s.t.Helper()

	fileID, size := s.upload(path)
	return models.Audio{
		FileID:       fileID,
		FileUniqueID: fileID + "-unique",
		Duration:     1,
		FileName:     filepath.Base(path),
		FileSize:     size,
	}
}

func (s *scenario) uploadDocument(path, mimeType string) models.Document {
	s.t.Helper()

	fileID, size := s.upload(path)
	return models.Document{
		FileID:       fileID,
		FileUniqueID: fileID + "-unique",
		FileName:     filepath.Base(path),
		MimeType:     mimeType,
		FileSize:     size,
	}
}

func (s *scenario) upload(path string) (fileID string, size int64) {
	s.t.Helper()

	s.uploads++
	fileID = fileIDFor(path, s.uploads)
	stored := s.botAPI.addFile(s.t, fileID, path)

	info, err := os.Stat(stored)
	require.NoError(s.t, err)
	return fileID, info.Size()
}

func (s *scenario) audioMessage(from telegramUser, audio models.Audio) *models.Update {
	return s.message(from, func(m *models.Message) { m.Audio = &audio })
}

func (s *scenario) documentMessage(from telegramUser, document models.Document) *models.Update {
	return s.message(from, func(m *models.Message) { m.Document = &document })
}

func (s *scenario) message(from telegramUser, fill func(*models.Message)) *models.Update {
	s.messages++
	msg := &models.Message{
		ID:   s.messages,
		From: &models.User{ID: from.ID, Username: from.Username},
		Chat: models.Chat{ID: from.ID, Type: models.ChatTypePrivate},
	}
	fill(msg)
	return &models.Update{ID: s.nextUpdateID(), Message: msg}
}

func (s *scenario) join(user telegramUser) {
	s.t.Helper()
	s.send(s.textMessage(user, "/start "+s.invite()))
}

func (s *scenario) textMessage(from telegramUser, text string) *models.Update {
	return s.message(from, func(m *models.Message) { m.Text = text })
}

func (s *scenario) link(from telegramUser, account navidromeAccount) *models.Update {
	msg := s.textMessage(from, "/link "+account.Login+" "+account.Password)
	s.send(msg)
	return msg
}

func (s *scenario) inlineQuery(from telegramUser, query string) *models.Update {
	id := s.nextUpdateID()
	return &models.Update{
		ID: id,
		InlineQuery: &models.InlineQuery{
			ID:    fmt.Sprintf("inline-%d", id),
			From:  &models.User{ID: from.ID, Username: from.Username},
			Query: query,
		},
	}
}

// press taps a button of the bot's message in the user's private chat.
func (s *scenario) press(from telegramUser, b button) *models.Update {
	update := s.callbackQuery(from, b.Data)
	update.CallbackQuery.Message = models.MaybeInaccessibleMessage{
		Type:    models.MaybeInaccessibleMessageTypeMessage,
		Message: &models.Message{ID: 1, Chat: models.Chat{ID: from.ID, Type: models.ChatTypePrivate}},
	}
	s.send(update)
	return update
}

// pressInline taps a button of a message the user sent through inline mode.
func (s *scenario) pressInline(from telegramUser, b button) *models.Update {
	update := s.callbackQuery(from, b.Data)
	update.CallbackQuery.InlineMessageID = "inline-message-" + update.CallbackQuery.ID
	s.send(update)
	return update
}

func (s *scenario) callbackQuery(from telegramUser, data string) *models.Update {
	id := s.nextUpdateID()
	return &models.Update{
		ID: id,
		CallbackQuery: &models.CallbackQuery{
			ID:   fmt.Sprintf("callback-%d", id),
			From: models.User{ID: from.ID, Username: from.Username},
			Data: data,
		},
	}
}

// Hidden entries are the bot's scratch space.
func (s *scenario) libraryFiles() []string {
	s.t.Helper()
	return filesUnder(s.t, s.library)
}

func filesUnder(t *testing.T, root string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && path == root {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && path != root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	require.NoError(t, err)
	slices.Sort(files)
	return files
}

func personalDir(user telegramUser) string {
	return filepath.Join("users", strconv.FormatInt(user.ID, 10))
}

func (s *scenario) personalFiles(user telegramUser) []string {
	s.t.Helper()
	return filesUnder(s.t, filepath.Join(s.library, personalDir(user)))
}

func (s *scenario) personalPath(user telegramUser, rel string) string {
	return filepath.Join(s.library, personalDir(user), rel)
}

func (s *scenario) sharedFiles() []string {
	s.t.Helper()
	return filesUnder(s.t, filepath.Join(s.library, "shared"))
}

func fixturePath(name string) string {
	return filepath.Join("testdata", "audio", name)
}
