package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lubaskinc0de/beatstash/e2e/harness"
	"github.com/lubaskinc0de/beatstash/e2e/harness/navidrome"
	"github.com/lubaskinc0de/beatstash/e2e/harness/telegram"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
	"github.com/lubaskinc0de/beatstash/internal/domain/library"
)

func TestAlbumListenLink(t *testing.T) {
	t.Parallel()

	t.Run("album name shows the Album before its tracks", func(t *testing.T) {
		s := harness.New(t)
		indexedOpera(t, s, alice)

		results := s.Search(alice, "opera", "").Results

		require.Len(t, results, 4)
		album := repositories.AlbumSummary{AlbumKey: library.AlbumKey{AlbumArtist: "Artist", Album: "Opera"}, Tracks: 3}
		assert.Equal(t, s.Catalog(alice).AlbumArticle(album).Title, results[0].Title)
		assert.Equal(t, s.Catalog(alice).AlbumArticle(album).Description, results[0].Description)
		assert.Equal(t, []string{"⏳"}, telegram.ButtonTexts(results[0].Buttons()))
		assert.Len(t, telegram.AudioFileIDs(telegram.InlineAnswer{Results: results[1:]}), 3)
	})

	t.Run("chosen Album is sent as a link to the Navidrome album", func(t *testing.T) {
		s := harness.New(t)
		account := indexedOpera(t, s, alice)

		chosen := s.Choose(alice, s.Search(alice, "opera", "").Results[0])

		shares := s.Navidrome.Shares(t, account)
		require.Len(t, shares, 1)
		assert.Equal(t, "album", shares[0].ResourceType)
		assert.Equal(t, "Artist — Opera", shares[0].Description)
		edits := s.Telegram.InlineEdits(t, chosen.ChosenInlineResult.InlineMessageID)
		require.Len(t, edits, 1)
		assert.Equal(t, s.Catalog(alice).AlbumLinkCaption("Artist", "Opera", shareURL(shares[0])), edits[0].Text)
	})

	t.Run("choosing the Album again gives the same link", func(t *testing.T) {
		s := harness.New(t)
		account := indexedOpera(t, s, alice)
		first := s.Choose(alice, s.Search(alice, "opera", "").Results[0])

		second := s.Choose(alice, s.Search(alice, "opera", "").Results[0])

		require.Len(t, s.Navidrome.Shares(t, account), 1)
		assert.Equal(t,
			s.Telegram.InlineEdits(t, first.ChosenInlineResult.InlineMessageID)[0].Text,
			s.Telegram.InlineEdits(t, second.ChosenInlineResult.InlineMessageID)[0].Text)
	})

	t.Run("track title shows no Album", func(t *testing.T) {
		s := harness.New(t)
		indexedOpera(t, s, alice)

		results := s.Search(alice, "song 2", "").Results

		require.Len(t, results, 1)
		assert.Equal(t, "audio", results[0].Type)
	})

	t.Run("Album Navidrome has not indexed is not shown", func(t *testing.T) {
		s := harness.New(t)
		s.LinkNewAccount(alice)
		s.UploadAlbum(alice, "Opera", 3)

		results := s.Search(alice, "opera", "").Results

		assert.Len(t, telegram.AudioFileIDs(telegram.InlineAnswer{Results: results}), 3)
		assert.Len(t, results, 3)
	})

	t.Run("Album is not shown without a public address", func(t *testing.T) {
		s := harness.New(t, harness.WithPublicURL(""))
		indexedOpera(t, s, alice)

		results := s.Search(alice, "opera", "").Results

		assert.Len(t, results, 3)
	})
}

// indexedOpera is "Artist — Opera" of 3 tracks whose songs the bot knows.
func indexedOpera(t *testing.T, s *harness.Scenario, user harness.User) navidrome.Account {
	t.Helper()

	account := s.LinkNewAccount(user)
	s.UploadAlbum(user, "Opera", 3)
	s.Navidrome.IndexedTrack(t, account, s.PersonalPath(user, ""), "Song 3")
	s.ResolveSongs()
	return account
}
