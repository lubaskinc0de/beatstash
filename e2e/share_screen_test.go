package e2e

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/navidrome-tg/e2e/harness"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/audiofile"
	"github.com/lubaskinc0de/navidrome-tg/e2e/harness/telegram"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

const (
	fixtureButton = "Fixture Artist — " + audiofile.FixtureTitle
	queenButton   = "Queen — Bohemian Rhapsody"
)

func TestShareScreenTracks(t *testing.T) {
	t.Parallel()

	t.Run("screen lists the newest uploads first", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Uploaded(alice, s.UploadAudioFile(queenSong(t)))

		s.Open(alice, s.Catalog(alice).ShareScreenButton())

		assert.Equal(t, s.Catalog(alice).ShareScreen("", true), s.WindowText())
		assert.Equal(t, []string{queenButton, fixtureButton}, ownButtons(t, s))
	})

	t.Run("typed query is deleted and the window shows what it found", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Uploaded(alice, s.UploadAudioFile(queenSong(t)))
		s.Open(alice, s.Catalog(alice).ShareScreenButton())

		query := s.SendText(alice, "queen")

		assert.Contains(t, s.Telegram.DeletedMessages(), strconv.Itoa(query.Message.ID))
		assert.Equal(t, s.Catalog(alice).ShareScreen("queen", true), s.WindowText())
		assert.Equal(t, []string{queenButton}, ownButtons(t, s))
	})

	t.Run("track the user only sees in the Shared Library is not found", func(t *testing.T) {
		s := harness.New(t)
		s.Share(bob, s.Uploaded(bob, s.UploadAudio("track.mp3")), s.Catalog(bob).ShareTrack())
		s.Open(alice, s.Catalog(alice).ShareScreenButton())

		s.SendText(alice, audiofile.FixtureTitle)

		assert.Equal(t, s.Catalog(alice).ShareScreen(audiofile.FixtureTitle, false), s.WindowText())
		assert.Empty(t, ownButtons(t, s))
	})

	t.Run("card shares the track and unshares it", func(t *testing.T) {
		s := harness.New(t)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		c := s.Catalog(alice)
		s.Open(alice, c.ShareScreenButton(), fixtureButton)

		s.Go(alice, c.ShareCardButton())
		shared := s.SharedFiles()
		s.Go(alice, c.UnshareCardButton())

		assert.Equal(t, []string{audiofile.FixtureTrackPath}, shared)
		assert.Empty(t, s.SharedFiles())
		assert.Contains(t, s.Telegram.CallbackAnswers(), c.ShareResult(&share_tracks.ShareResult{Created: 1}))
		assert.Contains(t, s.Telegram.CallbackAnswers(), c.Unshared())
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), c.ShareCardButton())
	})

	t.Run("card names who shared the track first", func(t *testing.T) {
		s := harness.New(t)
		mp3 := audiofile.Generate(t, "song.mp3", audiofile.Spec{Tags: audiofile.SongTags})
		s.Share(bob, s.Uploaded(bob, s.UploadAudioFile(mp3)), s.Catalog(bob).ShareTrack())
		s.Uploaded(alice, s.UploadAudioFile(mp3))

		s.Open(alice, s.Catalog(alice).ShareScreenButton(), "Artist — Dup Song")

		assert.Contains(t, s.WindowText(), s.Catalog(alice).SharedFirstBy(&access.User{Username: bob.Username}))
	})

	t.Run("card unshares an Attached Library track and sends no file", func(t *testing.T) {
		s, account, _ := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")})
		s.Link(alice, account)
		c := s.Catalog(alice)
		s.ShareOnScreen(alice, fixtureButton)

		s.Go(alice, c.UnshareCardButton())

		assert.Empty(t, s.SharedFiles())
		assert.Equal(t, []string{c.ShareCardButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("send file button sends the audio", func(t *testing.T) {
		s := harness.New(t)
		audio := s.UploadAudio("track.mp3")
		s.Uploaded(alice, audio)
		s.Open(alice, s.Catalog(alice).ShareScreenButton(), fixtureButton)

		s.Go(alice, s.Catalog(alice).SendOwnFileButton())

		sent := s.Telegram.CallsTo("sendAudio")
		require.Len(t, sent, 1)
		assert.Equal(t, audio.FileID, sent[0].Params["audio"])
		assert.Equal(t, strconv.FormatInt(alice.ID, 10), sent[0].Params["chat_id"])
	})

	t.Run("back leads to the same page", func(t *testing.T) {
		s := harness.New(t)
		uploadMany(t, s, alice, 11)
		c := s.Catalog(alice)
		s.Open(alice, c.ShareScreenButton(), c.NextPage())
		page := ownButtons(t, s)
		require.Len(t, page, 1)

		s.Go(alice, page[0], c.Back())

		assert.Equal(t, page, ownButtons(t, s))
	})
}

func ownButtons(t *testing.T, s *harness.Scenario) []string {
	t.Helper()

	var names []string
	for _, b := range s.Telegram.Buttons(t) {
		if strings.HasPrefix(b.Data, "go:share_") {
			names = append(names, b.Text)
		}
	}
	return names
}

func TestShareScreenAlbums(t *testing.T) {
	t.Parallel()

	t.Run("switching the mode keeps the query", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(alice, "Opera", 3)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		c := s.Catalog(alice)
		s.Open(alice, c.ShareScreenButton())
		s.SendText(alice, "opera")

		s.Go(alice, c.AlbumsMode())
		albums := ownButtons(t, s)
		s.Go(alice, c.TracksMode())

		assert.Equal(t, []string{c.OwnAlbumButton(operaAlbum())}, albums)
		assert.Equal(t, []string{"Artist — Song 1", "Artist — Song 2", "Artist — Song 3"}, ownButtons(t, s))
		assert.Equal(t, c.ShareScreen("opera", true), s.WindowText())
	})

	t.Run("album card shares the album and unshares it", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(alice, "Opera", 3)
		c := s.Catalog(alice)
		s.Open(alice, c.ShareScreenButton(), c.AlbumsMode(), c.OwnAlbumButton(operaAlbum()))

		s.Go(alice, c.ShareAlbumButton())
		shared := s.SharedFiles()
		s.Go(alice, c.UnshareAlbumButton())

		assert.Equal(t, []string{
			"Artist/Opera/01 - Song 1.mp3", "Artist/Opera/02 - Song 2.mp3", "Artist/Opera/03 - Song 3.mp3",
		}, shared)
		assert.Empty(t, s.SharedFiles())
		assert.Contains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), c.ShareAlbumButton())
	})

	t.Run("partly shared album card offers both", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(alice, "Opera", 3)
		c := s.Catalog(alice)
		s.ShareOnScreen(alice, "Artist — Song 2")

		s.Open(alice, c.ShareScreenButton(), c.AlbumsMode(), c.OwnAlbumButton(operaAlbum()))

		buttons := telegram.ButtonTexts(s.Telegram.Buttons(t))
		assert.Contains(t, buttons, c.ShareAlbumButton())
		assert.Contains(t, buttons, c.UnshareAlbumButton())
		assert.Equal(t, c.AlbumCard(&share_tracks.AlbumCard{
			Album:  operaAlbum().AlbumKey,
			Tracks: []library.Track{{ID: 1}, {ID: 2}, {ID: 3}},
			Shared: map[uint]bool{2: true},
		}), s.WindowText())
	})

	t.Run("track of the album card opens its card and leads back", func(t *testing.T) {
		s := harness.New(t)
		s.UploadAlbum(alice, "Opera", 3)
		c := s.Catalog(alice)
		s.Open(alice, c.ShareScreenButton(), c.AlbumsMode(), c.OwnAlbumButton(operaAlbum()))
		album := s.WindowText()

		s.Go(alice, "Artist — Song 2")
		card := s.WindowText()
		s.Go(alice, c.Back())

		assert.Contains(t, card, "Song 2")
		assert.Contains(t, card, c.TrackCard(&share_tracks.TrackCard{Track: &library.Track{Metadata: library.Metadata{
			Artist: "Artist", Title: "Song 2", Album: "Opera",
		}}}))
		assert.Equal(t, album, s.WindowText())
	})

	t.Run("listen link comes as a message and is the inline one", func(t *testing.T) {
		s := harness.New(t)
		account := indexedOpera(t, s, alice)
		c := s.Catalog(alice)
		s.Open(alice, c.ShareScreenButton(), c.AlbumsMode(), c.OwnAlbumButton(operaAlbum()))
		window := s.WindowText()

		s.Go(alice, c.ListenLinkButton())
		chosen := s.Choose(alice, s.Search(alice, "opera", "").Results[0])

		shares := s.Navidrome.Shares(t, account)
		require.Len(t, shares, 1)
		link := c.AlbumLinkCaption("Artist", "Opera", shareURL(shares[0]))
		assert.Equal(t, []string{link}, s.SentMessagesContaining(alice, shareURL(shares[0])))
		assert.Equal(t, link, s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)[0].Text)
		assert.Equal(t, window, s.WindowText())
	})

	t.Run("album card has no listen link without a public address", func(t *testing.T) {
		s := harness.New(t, harness.WithPublicURL(""))
		indexedOpera(t, s, alice)
		c := s.Catalog(alice)

		s.Open(alice, c.ShareScreenButton(), c.AlbumsMode(), c.OwnAlbumButton(operaAlbum()))

		assert.NotContains(t, telegram.ButtonTexts(s.Telegram.Buttons(t)), c.ListenLinkButton())
	})
}

func operaAlbum() repositories.AlbumSummary {
	return repositories.AlbumSummary{AlbumKey: library.AlbumKey{AlbumArtist: "Artist", Album: "Opera"}, Tracks: 3}
}
