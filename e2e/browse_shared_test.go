package e2e

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
)

func TestTake(t *testing.T) {
	t.Parallel()

	t.Run("stale Take button says the track is no longer shared", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		s.OpenShared(bob, 1)
		take := s.Button(s.Catalog(bob).TakeButton())

		s.UnshareTrack(alice, fixtureButton)
		s.Press(bob, take)

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(bob).NotShared())
		assert.Empty(t, s.PersonalFiles(bob))
	})

	t.Run("stale Send File button says the track is no longer shared", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		s.OpenShared(bob, 1)
		sendFile := s.Button(s.Catalog(bob).SendFileButton())

		s.UnshareTrack(alice, fixtureButton)
		s.Press(bob, sendFile)

		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(bob).NotShared())
		assert.Empty(t, s.Telegram.CallsTo("sendAudio"))
		assert.Empty(t, s.PersonalFiles(bob))
	})

	t.Run("taken track lands in the taker's Personal Library", func(t *testing.T) {
		s := harness.New(t)
		bobAccount := s.LinkNewAccount(bob)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.Take(bob, 1)

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
		s.Navidrome.IndexedTrack(t, bobAccount, s.PersonalPath(bob, ""), audiofile.FixtureTitle)
		assert.Contains(t, s.LastCallbackAnswer(), s.Catalog(bob).Taken())
	})

	t.Run("taken track stays after the author unshares", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		s.Take(bob, 1)

		s.UnshareTrack(alice, fixtureButton)

		assert.Empty(t, s.SharedFiles())
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
	})

	t.Run("taker reshares after the author unshares", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, audio)
		s.ShareTrack(alice, fixtureButton)
		s.Take(bob, 1)
		s.UnshareTrack(alice, fixtureButton)

		s.ShareTrack(bob, fixtureButton)

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.SharedFiles())
		assert.Contains(t, newestShare(s, alice), "@bob")
		assert.NotContains(t, newestShare(s, alice), "@alice")
	})

	t.Run("upload of a shared file needs no download", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, audio)
		s.ShareTrack(alice, fixtureButton)
		s.Telegram.Forget()
		forward := s.AudioMessage(bob, audio)

		s.Send(forward)
		s.WaitIngest()

		assert.Empty(t, s.Telegram.CallsTo("getFile"))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
		assert.Equal(t, []string{"👀", "👍"}, s.Telegram.ReactionsOn(t, forward.Message.ID))
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("shared file of a track the user owns is not copied", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Uploaded(bob, s.UploadAudioFile(mp3))
		aliceAudio := s.UploadAudioFile(mp3)
		s.Uploaded(alice, aliceAudio)
		s.ShareTrack(alice, dupSongButton)
		s.Telegram.Forget()
		forward := s.AudioMessage(bob, aliceAudio)

		s.Send(forward)
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.PersonalFiles(bob))
		harness.AssertAlreadyExists(t, s, bob, forward.Message.ID)
	})

	t.Run("user's own version of a shared track is stored", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Uploaded(alice, s.UploadAudioFile(mp3))
		s.ShareTrack(alice, dupSongButton)
		flac := s.UploadDocument(audiofile.Generate(t, "song.flac", audiofile.Spec{Tags: audiofile.SongTags}), "audio/flac")
		s.Telegram.Forget()

		s.Send(s.DocumentMessage(bob, flac))
		s.WaitIngest()

		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.flac"}, s.PersonalFiles(bob))
		assert.Equal(t, []string{"Artist/Album/01 - Dup Song.mp3"}, s.SharedFiles())
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("upload of a track private to another user reveals nothing", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Telegram.Forget()

		s.Uploaded(bob, s.UploadAudio("track.mp3"))

		assert.NotEmpty(t, s.Telegram.CallsTo("getFile"))
		assert.Equal(t, []string{audiofile.FixtureTrackPath}, s.PersonalFiles(bob))
		assert.Empty(t, s.Telegram.Replies(t))
	})

	t.Run("send file button sends the audio", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, audio)
		s.ShareTrack(alice, fixtureButton)
		s.OpenShared(bob, 1)

		s.Press(bob, s.Button(s.Catalog(bob).SendFileButton()))

		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, audio.FileID, sent[0].Params["audio"])
		assert.Equal(t, "1002", sent[0].Params["chat_id"])
	})
}

func TestSharedFeed(t *testing.T) {
	t.Parallel()

	t.Run("inline shared lists recent Shares with their authors", func(t *testing.T) {
		s := harness.New(t)
		aliceAudio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, aliceAudio)
		s.ShareTrack(alice, fixtureButton)
		bobAudio := s.UploadAudioFile(audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags}))
		s.Uploaded(bob, bobAudio)
		s.ShareTrack(bob, dupSongButton)
		query := s.InlineQuery(alice, "shared")

		s.Send(query)

		results := s.Telegram.InlineAnswerTo(t, query).Results
		require.Len(t, results, 3)
		assert.Equal(t, bobAudio.FileID, results[1].AudioFileID)
		assert.Contains(t, results[1].Caption, "@bob")
		assert.Equal(t, aliceAudio.FileID, results[2].AudioFileID)
		assert.Contains(t, results[2].Caption, "@alice")
	})

	t.Run("chosen shared FLAC becomes its audio signed by the author", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.flac")
		s.Uploaded(alice, audio)
		s.ShareTrack(alice, fixtureButton)
		results := s.Search(bob, "shared", "").Results
		require.Len(t, results, 2)

		chosen := s.Choose(bob, results[1])

		assert.Contains(t, results[1].Content.Text, "⏳")
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageMedia", edits[0].Method)
		assert.Equal(t, audio.FileID, edits[0].Media.Media)
		assert.Contains(t, edits[0].Media.Caption, audiofile.FixtureTitle)
		assert.Contains(t, edits[0].Media.Caption, "@alice")
	})

	t.Run("chosen track from inline shared goes through the storage chat with its author", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill())
		sharedZvukSong(t, s, alice, "Feed Song")
		results := s.Search(bob, "shared", "").Results
		require.Len(t, results, 2)

		chosen := s.Choose(bob, results[1])

		assert.Contains(t, results[1].Content.Text, "⏳")
		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, strconv.Itoa(storageChat), sent[0].Params["chat_id"])
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, s.Telegram.UploadedFileID(0), edits[0].Media.Media)
		assert.Contains(t, edits[0].Media.Caption, "Feed Song")
		assert.Contains(t, edits[0].Media.Caption, "@alice")
	})

	t.Run("track unshared before the choice is not sent", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.flac"))
		s.ShareTrack(alice, fixtureButton)
		results := s.Search(bob, "shared", "").Results
		require.Len(t, results, 2)
		s.UnshareTrack(alice, fixtureButton)

		chosen := s.Choose(bob, results[1])

		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, "editMessageText", edits[0].Method)
		assert.Equal(t, s.Catalog(bob).NotSentNote(false), edits[0].Text)
	})

	t.Run("Share made on one instance is in another's feed", func(t *testing.T) {
		s := harness.New(t)
		replica := s.StartReplica()
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		replica.Open(bob, s.Catalog(bob).MusicButton())

		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), fixtureButton)
	})

	t.Run("empty feed says nobody has shared yet", func(t *testing.T) {
		s := harness.New(t)

		s.Open(alice, s.Catalog(alice).MusicButton())

		empty := s.Catalog(alice).FeedEmpty()
		require.NotEmpty(t, empty)
		assert.Equal(t, empty, s.WindowText())
	})

	t.Run("taken track shows as the user's", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.Take(bob, 1)

		c := s.Catalog(bob)
		assert.Equal(t, "editMessageText", s.Telegram.Window().Method)
		assert.Equal(t, []string{c.InLibraryButton(), c.SendFileButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("author sees their own Share as theirs", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)

		s.OpenShared(alice, 1)

		c := s.Catalog(alice)
		assert.Equal(t, []string{c.InLibraryButton(), c.SendFileButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("feed and top commands are gone", func(t *testing.T) {
		s := harness.New(t)

		shared := s.TextMessage(alice, "/shared")
		top := s.TextMessage(alice, "/top")
		s.Send(shared)
		s.Send(top)

		assertOnlyDeleted(t, s, shared, top)
	})
}

func newestShare(s *harness.Scenario, user harness.User) string {
	s.OpenShared(user, 1)
	return s.WindowText()
}

func TestAuthorName(t *testing.T) {
	t.Parallel()

	t.Run("admin from config is shown by username", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(admin, s.UploadAudio("track.mp3"))
		s.ShareTrack(admin, fixtureButton)

		assert.Contains(t, newestShare(s, alice), "@"+admin.Username)
	})

	t.Run("renamed user is shown by the new username", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.ShareTrack(alice, fixtureButton)
		renamed := harness.User{ID: alice.ID, Username: "alice_new"}

		s.Send(s.TextMessage(renamed, "/start"))

		assert.Contains(t, newestShare(s, bob), "@alice_new")
	})
}
