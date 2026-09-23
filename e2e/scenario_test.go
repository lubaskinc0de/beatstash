package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/internal/app"
	"github.com/lubaskinc0de/navidrome-tg/internal/config"
)

const (
	fixtureArtist = "Fixture Artist"
	fixtureTitle  = "Fixture Song"
)

type telegramUser struct {
	ID       int64
	Username string
}

var (
	allowedUser = telegramUser{ID: 1001, Username: "alice"}
	stranger    = telegramUser{ID: 6666, Username: "mallory"}
)

var databaseSeq atomic.Int64

type scenario struct {
	t       *testing.T
	bot     *bot.Bot
	botAPI  *botAPI
	library string

	updates  int64
	messages int
	uploads  int
}

func newScenario(t *testing.T) *scenario {
	t.Helper()

	api := newBotAPI(t)

	library, err := os.MkdirTemp(env.libraryRoot, "scenario-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(library, 0o755))

	b, err := app.New(
		config.Config{
			Token:             botToken,
			BotApiUrl:         api.URL(),
			DbDsn:             postgresDSN(createDatabase(t)),
			MusicDir:          library,
			AllowedUserIds:    []uint64{uint64(allowedUser.ID)},
			NavidromeUser:     navidromeAdmin,
			NavidromePassword: navidromePassword,
			NavidromeUrl:      env.navidrome.url,
		},
		bot.WithSkipGetMe(),
		bot.WithNotAsyncHandlers(),
	)
	require.NoError(t, err)

	return &scenario{t: t, bot: b, botAPI: api, library: library}
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
	s.bot.ProcessUpdate(s.t.Context(), update)
}

func (s *scenario) nextUpdateID() int64 {
	s.updates++
	return s.updates
}

// uploadAudio puts a fixture on the Bot API double, as if the user had
// uploaded it to Telegram, and returns the audio as Telegram describes it.
func (s *scenario) uploadAudio(fixture string) models.Audio {
	s.t.Helper()

	path := fixturePath(fixture)
	info, err := os.Stat(path)
	require.NoError(s.t, err)

	s.uploads++
	fileID := fileIDFor(fixture, s.uploads)
	s.botAPI.addFile(fileID, path)

	return models.Audio{
		FileID:       fileID,
		FileUniqueID: fileID + "-unique",
		Duration:     1,
		Performer:    fixtureArtist,
		Title:        fixtureTitle,
		FileName:     fixture,
		FileSize:     info.Size(),
	}
}

func (s *scenario) audioMessage(from telegramUser, audio models.Audio) *models.Update {
	s.messages++
	return &models.Update{
		ID: s.nextUpdateID(),
		Message: &models.Message{
			ID:    s.messages,
			From:  &models.User{ID: from.ID, Username: from.Username},
			Chat:  models.Chat{ID: from.ID, Type: models.ChatTypePrivate},
			Audio: &audio,
		},
	}
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

// libraryChecksums returns sha256 of every file in the scenario's Library.
func (s *scenario) libraryChecksums() []string {
	s.t.Helper()

	var sums []string
	err := filepath.WalkDir(s.library, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		sums = append(sums, checksum(s.t, path))
		return nil
	})
	require.NoError(s.t, err)
	return sums
}

func fixturePath(name string) string {
	return filepath.Join("testdata", "audio", name)
}

func checksum(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
