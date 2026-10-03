package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

const (
	kb           = library.Quota(1 << 10)
	adminContact = "@boss_contact"
)

func TestUploadQuota(t *testing.T) {
	t.Parallel()

	t.Run("upload over the Quota is rejected and stays out of the Library", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(20*kb), harness.WithAdminContact(adminContact))
		upload := s.AudioMessage(alice, s.UploadAudio("track.mp3"))

		s.Send(upload)
		s.WaitIngest()

		assert.Empty(t, s.PersonalFiles(alice))
		assert.Equal(t, []string{"👀", "👎"}, s.Telegram.ReactionsOn(t, upload.Message.ID))
		reply := s.LastReply()
		assert.Equal(t, upload.Message.ID, reply.ReplyTo)
		assert.Equal(t, s.Catalog(alice).NoRoom(library.Usage{Quota: 20 * kb}, adminContact), reply.Text)
	})

	t.Run("replacing a Duplicate with better Quality counts the difference", func(t *testing.T) {
		low := audiofile.Generate(t, "low.mp3", audiofile.Spec{Bitrate: "128k", Tags: audiofile.SongTags})
		high := audiofile.Generate(t, "high.mp3", audiofile.Spec{Bitrate: "320k", Tags: audiofile.SongTags})
		quota := library.Quota(harness.FileSize(t, high) + harness.FileSize(t, low)/2)
		s := harness.New(t, harness.WithDefaultQuota(quota))
		s.Uploaded(alice, s.UploadAudioFile(low))
		better := s.AudioMessage(alice, s.UploadAudioFile(high))

		s.Send(better)
		s.WaitIngest()

		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, better.Message.ID))
		assert.Equal(t, 320, audiofile.BitrateKbps(t, s.PersonalPath(alice, "Artist/Album/01 - Dup Song.mp3")))
	})

	t.Run("two uploads at once do not exceed the Quota together", func(t *testing.T) {
		first := audiofile.Generate(t, "first.mp3", audiofile.Spec{Tags: map[string]string{"artist": "A", "title": "First"}})
		second := audiofile.Generate(t, "second.mp3", audiofile.Spec{Tags: map[string]string{"artist": "A", "title": "Second"}})
		s := harness.New(t, harness.WithDefaultQuota(library.Quota(harness.FileSize(t, first)*3/2)))

		s.Send(s.AudioMessage(alice, s.UploadAudioFile(first)))
		s.Send(s.AudioMessage(alice, s.UploadAudioFile(second)))
		s.WaitIngest()

		assert.Len(t, s.PersonalFiles(alice), 1)
		assert.ElementsMatch(t, []string{"👀", "👀", "👍", "👎"}, s.Telegram.Reactions(t))
	})

	t.Run("Home shows the Usage of a limited Quota", func(t *testing.T) {
		s := harness.New(t, harness.WithDefaultQuota(100*kb))
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Open(alice)

		used := harness.FileSize(t, s.PersonalPath(alice, audiofile.FixtureTrackPath))
		usage := library.Usage{Used: used, Quota: 100 * kb}
		assert.Contains(t, s.WindowText(), s.Catalog(alice).Home(alice.Username, telegram.BotUsername, usage))
	})

	t.Run("without a Quota nothing is limited and Home tells no Usage", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))

		s.Open(alice)

		require.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(alice))
		assert.Equal(t, s.Catalog(alice).Home(alice.Username, telegram.BotUsername, library.Usage{}), s.WindowText())
	})
}
