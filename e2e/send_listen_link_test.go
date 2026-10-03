package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/audiofile"
	"github.com/lubaskinc0de/beatstash/e2e/harness/navidrome"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	"github.com/lubaskinc0de/beatstash/internal/infra/telegram/i18n"
)

func TestTrackListenLink(t *testing.T) {
	t.Parallel()

	t.Run("track without a Telegram file is sent as its Listen Link", func(t *testing.T) {
		s := harness.New(t)
		account := linkedZvukSong(t, s, alice, "Linked Song")
		result := onlyResult(t, s, alice, "linked song")

		chosen := s.Choose(alice, result)

		assert.Contains(t, result.Content.Text, "⏳")
		shares := s.Navidrome.Shares(t, account)
		require.Len(t, shares, 1)
		assert.Equal(t, "Zvuk Band — Linked Song", shares[0].Description)
		assert.True(t, shares[0].Downloadable)
		assert.WithinDuration(t, time.Now().Add(harness.ListenLinkTTL), shares[0].ExpiresAt, time.Minute)
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, s.Catalog(alice).ListenLinkCaption(i18n.TrackCaption("Zvuk Band", "Linked Song"), shareURL(shares[0])), edits[0].Text)
		assert.Empty(t, edits[0].Buttons)
	})

	t.Run("choosing the track again gives the same link", func(t *testing.T) {
		s := harness.New(t)
		account := linkedZvukSong(t, s, alice, "Linked Song")
		first := s.Choose(alice, onlyResult(t, s, alice, "linked song"))

		second := s.Choose(alice, onlyResult(t, s, alice, "linked song"))

		require.Len(t, s.Navidrome.Shares(t, account), 1)
		assert.Equal(t,
			s.Telegram.InlineEdits(t, first.ChosenInlineResult.InlineMessageID)[0].Text,
			s.Telegram.InlineEdits(t, second.ChosenInlineResult.InlineMessageID)[0].Text)
	})

	t.Run("Attached Library track is sent as its Listen Link", func(t *testing.T) {
		s, account, _ := newWithOwnLibrary(t, []string{audiofile.Fixture("track.mp3")})
		s.Link(alice, account)

		chosen := s.Choose(alice, onlyResult(t, s, alice, audiofile.FixtureTitle))

		shares := s.Navidrome.Shares(t, account)
		require.Len(t, shares, 1)
		assert.Contains(t, s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)[0].Text, shareURL(shares[0]))
	})

	t.Run("file too large for Telegram is sent as its Listen Link", func(t *testing.T) {
		s := harness.New(t, harness.WithStorageChat(storageChat), harness.WithoutStorageFill(), harness.WithMaxUpload(tooSmall))
		account := linkedZvukSong(t, s, alice, "Huge Song")

		chosen := s.Choose(alice, onlyResult(t, s, alice, "huge song"))

		shares := s.Navidrome.Shares(t, account)
		require.Len(t, shares, 1)
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, s.Catalog(alice).ListenLinkCaption(i18n.TrackCaption("Zvuk Band", "Huge Song"), shareURL(shares[0])), edits[0].Text)
	})

	for name, url := range map[string]string{
		"without a public address":            "",
		"with a local public address":         "http://localhost:4533",
		"with a public address of a LAN host": "http://navidrome:4533",
	} {
		t.Run("track that can be sent no way is left out "+name, func(t *testing.T) {
			s := harness.New(t, harness.WithPublicURL(url))
			linkedZvukSong(t, s, alice, "Plain Song")

			answer := s.Search(alice, "plain song", "")

			assert.Empty(t, answer.Results)
			assert.Equal(t, s.Catalog(alice).UnsendableButton(), answer.Button.Text)
		})
	}

	t.Run("button over left out tracks tells of the missing public address", func(t *testing.T) {
		s := harness.New(t, harness.WithPublicURL(""))
		linkedZvukSong(t, s, alice, "Plain Song")
		button := s.Search(alice, "plain song", "").Button

		s.Send(s.TextMessage(alice, "/start "+button.StartParameter))

		c := s.Catalog(alice)
		assert.Equal(t, c.UnsendableNoAddress(), s.WindowText())
		assert.Equal(t, []string{c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("button over left out tracks asks to link a Navidrome Account", func(t *testing.T) {
		s := harness.New(t)
		importZvukSong(s, alice, "Plain Song")
		button := s.Search(alice, "plain song", "").Button

		s.Send(s.TextMessage(alice, "/start "+button.StartParameter))

		c := s.Catalog(alice)
		assert.Equal(t, c.UnsendableNoAccount(), s.WindowText())
		assert.Equal(t, []string{c.AccountsButton(), c.Back()}, telegram.ButtonTexts(s.Telegram.Buttons(t)))
	})

	t.Run("Navidrome without sharing leaves text with a note", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidromeWithoutSharing(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
		})
		s.Link(alice, account)
		importZvukSong(s, alice, "Unshared Song")
		s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Unshared Song")

		chosen := s.Choose(alice, onlyResult(t, s, alice, "unshared song"))

		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		c := s.Catalog(alice)
		assert.Equal(t, c.LinkFailedCaption(i18n.TrackCaption("Zvuk Band", "Unshared Song")), edits[0].Text)
	})

	t.Run("track Navidrome has not indexed is not ready", func(t *testing.T) {
		var account navidrome.Account
		s := harness.NewOwnNavidrome(t, func(s *harness.Scenario) {
			account = s.Navidrome.CreateAccount(t, "alice")
		})
		s.Link(alice, account)
		importZvukSong(s, alice, "Fresh Song")

		chosen := s.Choose(alice, onlyResult(t, s, alice, "fresh song"))

		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		c := s.Catalog(alice)
		assert.Equal(t, c.NotIndexedCaption(i18n.TrackCaption("Zvuk Band", "Fresh Song")), edits[0].Text)
		assert.Empty(t, s.Navidrome.Shares(t, account))
	})
}

// linkedZvukSong has no Telegram file.
func linkedZvukSong(t *testing.T, s *harness.Scenario, user harness.User, title string) navidrome.Account {
	t.Helper()

	account := s.LinkNewAccount(user)
	importZvukSong(s, user, title)
	s.Navidrome.IndexedTrack(t, account, s.PersonalPath(user, ""), title)
	return account
}

func onlyResult(t *testing.T, s *harness.Scenario, user harness.User, text string) telegram.InlineResult {
	t.Helper()

	results := s.Search(user, text, "").Results
	require.Len(t, results, 1)
	return results[0]
}

func shareURL(share navidrome.Share) string {
	return harness.NavidromePublicURL + "/share/" + share.ID
}

func TestPlayingListenLink(t *testing.T) {
	t.Parallel()

	t.Run("np offers no Share", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		s.Uploaded(alice, s.UploadAudio("track.mp3"))
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), audiofile.FixtureTitle).ID)

		result := s.NowPlaying(alice)

		assert.Equal(t, "audio", result.Type)
		assert.Empty(t, result.Buttons())
	})

	t.Run("np of a track without a Telegram file is sent as its Listen Link", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		importZvukSong(s, alice, "Playing Song")
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Playing Song").ID)
		result := s.NowPlaying(alice)

		chosen := s.Choose(alice, result)

		assert.Equal(t, []string{"⏳"}, telegram.ButtonTexts(result.Buttons()))
		shares := s.Navidrome.Shares(t, account)
		require.Len(t, shares, 1)
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		np := nowPlayingText(t, s, alice, result, "Zvuk Band", "Playing Song", "Zvuk Album")
		assert.Equal(t, s.Catalog(alice).ListenLinkCaption(np, shareURL(shares[0])), edits[0].Text)
	})

	t.Run("recent leaves out a track it can send no way", func(t *testing.T) {
		s := harness.New(t, harness.WithPublicURL(""))
		account := s.LinkNewAccount(alice)
		importZvukSong(s, alice, "Played Song")
		s.Navidrome.Play(t, account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Played Song").ID)

		answer := s.Search(alice, "recent", "")

		require.Len(t, answer.Results, 1)
		assert.Equal(t, "recent-list", answer.Results[0].ID)
		assert.Equal(t, s.Catalog(alice).UnsendableButton(), answer.Button.Text)
	})

	t.Run("np of a track it can send no way still tells what is playing", func(t *testing.T) {
		s := harness.New(t, harness.WithPublicURL(""))
		account := s.LinkNewAccount(alice)
		importZvukSong(s, alice, "Playing Song")
		s.Navidrome.StartPlaying(t, account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Playing Song").ID)

		result := s.NowPlaying(alice)

		assert.Equal(t, "article", result.Type)
		assert.Contains(t, result.Content.Text, "Playing Song")
		assert.Empty(t, result.Buttons())
	})

	t.Run("recent track without a Telegram file is sent as its Listen Link", func(t *testing.T) {
		s := harness.New(t)
		account := s.LinkNewAccount(alice)
		importZvukSong(s, alice, "Played Song")
		s.Navidrome.Play(t, account, s.Navidrome.IndexedTrack(t, account, s.PersonalPath(alice, ""), "Played Song").ID)
		results := s.Search(alice, "recent", "").Results
		require.Len(t, results, 2)

		chosen := s.Choose(alice, results[1])

		shares := s.Navidrome.Shares(t, account)
		require.Len(t, shares, 1)
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, s.Catalog(alice).ListenLinkCaption(i18n.TrackCaption("Zvuk Band", "Played Song"), shareURL(shares[0])), edits[0].Text)
	})
}
