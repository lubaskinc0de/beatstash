package i18n

import (
	"path"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/access"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
)

func (c Catalog) TracksMode() string        { return c.t("share_screen.tracks", nil) }
func (c Catalog) AlbumsMode() string        { return c.t("share_screen.albums", nil) }
func (c Catalog) ShareCardButton() string   { return c.t("share_screen.share", nil) }
func (c Catalog) UnshareCardButton() string { return c.t("share_screen.unshare", nil) }
func (c Catalog) SendOwnFileButton() string { return c.t("share_screen.send_file", nil) }
func (c Catalog) ShareAlbumButton() string  { return c.t("share_screen.share_album", nil) }
func (c Catalog) UnshareAlbumButton() string {
	return c.t("share_screen.unshare_album", nil)
}
func (c Catalog) ListenLinkButton() string { return c.t("share_screen.listen_link", nil) }
func (c Catalog) ListenLinkSent() string   { return c.t("share_screen.sent", nil) }

func (c Catalog) ShareScreen(query string, found bool) string {
	text := c.t("share_screen.title", nil) + "\n\n" + esc(c.t("share_screen.prompt", nil)) + "\n\n"
	q := args{"Query": esc(query)}
	switch {
	case query == "" && found:
		return text + c.t("share_screen.newest", nil)
	case query == "":
		return text + c.t("share_screen.empty", nil)
	case found:
		return text + c.t("share_screen.found", q)
	default:
		return text + c.t("share_screen.nothing", q)
	}
}

// OwnTrackButton is plain text.
func (c Catalog) OwnTrackButton(track *library.Track) string {
	return track.Artist + " — " + track.Title
}

// OwnAlbumButton is plain text.
func (c Catalog) OwnAlbumButton(album repositories.AlbumSummary) string {
	return AlbumTitle(album.AlbumArtist, album.Album) + " · " + c.tracks(album.Tracks)
}

func (c Catalog) TrackCard(card *share_tracks.TrackCard) string {
	track := card.Track
	text := TrackCaption(track.Artist, track.Title)
	if !track.Single() {
		text += "\n💿 " + esc(track.Album)
	}
	text += "\n📚 " + esc(c.libraryName(card.Library)) + "\n\n"
	switch {
	case card.Author != nil:
		return text + c.SharedFirstBy(card.Author)
	case card.Shared:
		return text + c.t("share_screen.shared", nil)
	default:
		return text + c.t("share_screen.not_shared", nil)
	}
}

func (c Catalog) SharedFirstBy(author *access.User) string {
	return c.t("share_screen.shared_by", args{"Author": esc(c.authorName(author))})
}

func (c Catalog) AlbumCard(card *share_tracks.AlbumCard) string {
	return AlbumCaption(card.Album.AlbumArtist, card.Album.Album) + "\n" + c.tracks(len(card.Tracks)) + "\n📚 " +
		esc(c.libraryName(card.Library)) + "\n\n" +
		c.t("share_screen.album_shared", args{"Shared": card.SharedCount(), "Tracks": len(card.Tracks)})
}

func (c Catalog) libraryName(lib *library.Library) string {
	if lib == nil || !lib.Attached() {
		return c.t("share_screen.personal", nil)
	}
	return path.Base(lib.Dir)
}

func (c Catalog) UnsendableButton() string    { return c.t("unsendable.button", nil) }
func (c Catalog) UnsendableNoAccount() string { return c.t("unsendable.no_account", nil) }
func (c Catalog) UnsendableNoAddress() string { return c.t("unsendable.no_address", nil) }
func (c Catalog) ListenLinksReady() string    { return c.t("unsendable.ready", nil) }
