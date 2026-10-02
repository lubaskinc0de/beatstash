package bot

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/search_music"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/send_listen_link"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

type ShareScreen struct {
	Telegram           *Telegram
	Users              *Users
	SearchOwnTracks    *search_music.SearchOwnTracks
	SearchOwnAlbums    *search_music.SearchOwnAlbums
	ViewTrackCard      *share_tracks.ViewTrackCard
	ViewAlbumCard      *share_tracks.ViewAlbumCard
	ShareTrack         *share_tracks.ShareTrack
	ShareAlbum         *share_tracks.ShareAlbum
	UnshareTrack       *share_tracks.UnshareTrack
	UnshareAlbum       *share_tracks.UnshareAlbum
	GetTrackFile       *show_playing.GetTrackFile
	GetAlbumListenLink *send_listen_link.GetAlbumListenLink
	Files              *trackfile.Files
}

const ownPage = 10

const (
	actionShareCard        = "sc"
	actionUnshareCard      = "uc"
	actionShareAlbumCard   = "sa"
	actionUnshareAlbumCard = "ua"
	actionSendOwnFile      = "sf"
	actionSendAlbumLink    = "sl"
)

// ownList's arg is the mode's letter and the page; an empty arg opens the
// screen anew.
type ownList struct {
	albums bool
	page   int
}

func parseOwnList(arg string) ownList {
	if arg == "" {
		return ownList{}
	}
	page, _ := strconv.Atoi(arg[1:])
	return ownList{albums: arg[0] == 'a', page: page}
}

func (l ownList) arg() string {
	mode := "t"
	if l.albums {
		mode = "a"
	}
	return mode + strconv.Itoa(l.page)
}

func (l ownList) place() place {
	return place{screen: screenShare, arg: l.arg()}
}

// A card's arg is "<track id>:<back>", back the arg of the list it came
// from, or "@" and the arg of an Album card.
func cardArg(trackID uint, back string) string {
	return strconv.FormatUint(uint64(trackID), 10) + ":" + back
}

func parseCardArg(arg string) (trackID uint, back place) {
	id, rest, _ := strings.Cut(arg, ":")
	n, _ := strconv.ParseUint(id, 10, 64)
	if album, ok := strings.CutPrefix(rest, "@"); ok {
		return uint(n), place{screen: screenShareAlbum, arg: album}
	}
	return uint(n), parseOwnList(rest).place()
}

func (s *ShareScreen) shareView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	list := parseOwnList(arg)
	query, err := s.query(ctx, arg == "")
	if err != nil {
		slog.Error("share_query", "error", err)
	}
	tabs := musicTabs(c, screenShare)
	found, more, err := s.ownRows(ctx, c, list, query)
	if err != nil {
		slog.Error("find_own_music", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{tabs, back}}
	}
	text := c.ShareScreen(query, len(found) > 0)
	rows := append([][]models.InlineKeyboardButton{tabs}, found...)
	if pages := pageRow(c, list.page, more, func(page int) place { return ownList{albums: list.albums, page: page}.place() }); pages != nil {
		rows = append(rows, pages)
	}
	return window.View{Text: text, Rows: append(rows, modeRow(c, list), back)}
}

// query is forgotten by a screen opened anew.
func (s *ShareScreen) query(ctx context.Context, anew bool) (string, error) {
	if anew {
		return "", s.Users.SetShareQuery(ctx, senderID(ctx), "")
	}
	return s.Users.ShareQuery(ctx, senderID(ctx))
}

func (s *ShareScreen) ownRows(
	ctx context.Context, c i18n.Catalog, list ownList, query string,
) (rows [][]models.InlineKeyboardButton, more bool, err error) {
	if list.albums {
		found, err := s.SearchOwnAlbums.Execute(ctx, query, list.page*ownPage, ownPage)
		if err != nil {
			return nil, false, err
		}
		for _, album := range found.Found {
			rows = append(rows, []models.InlineKeyboardButton{
				goButton(c.OwnAlbumButton(album), place{screen: screenShareAlbum, arg: cardArg(album.TrackID, list.arg())}),
			})
		}
		return rows, found.More, nil
	}
	found, err := s.SearchOwnTracks.Execute(ctx, query, list.page*ownPage, ownPage)
	if err != nil {
		return nil, false, err
	}
	for n := range found.Found {
		track := &found.Found[n]
		rows = append(rows, []models.InlineKeyboardButton{
			goButton(c.OwnTrackButton(track), place{screen: screenShareTrack, arg: cardArg(track.ID, list.arg())}),
		})
	}
	return rows, found.More, nil
}

func modeRow(c i18n.Catalog, list ownList) []models.InlineKeyboardButton {
	tracks, albums := c.TracksMode(), c.AlbumsMode()
	if list.albums {
		albums = "• " + albums
	} else {
		tracks = "• " + tracks
	}
	return []models.InlineKeyboardButton{
		goButton(tracks, ownList{}.place()),
		goButton(albums, ownList{albums: true}.place()),
	}
}

func (s *ShareScreen) typeQuery(ctx context.Context, in windowInput) {
	if err := s.Users.SetShareQuery(ctx, in.chatID, in.text); err != nil {
		slog.Error("save_share_query", "error", err)
	}
	list := parseOwnList(in.arg)
	list.page = 0
	s.Telegram.show(ctx, in.chatID, window.Current, list.place(), "")
}

func (s *ShareScreen) trackCardView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	trackID, back := parseCardArg(arg)
	card, err := s.ViewTrackCard.Execute(ctx, trackID)
	if errors.Is(err, library.ErrNotKeptTrack) {
		return s.Telegram.screens.view(ctx, back.screen, back.arg).WithNotice(c.NotOwnTrack())
	}
	if err != nil {
		slog.Error("view_track_card", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{backRow(ctx, back)}}
	}
	share := models.InlineKeyboardButton{Text: c.ShareCardButton(), CallbackData: actionShareCard + ":" + arg, Style: stylePrimary}
	if card.Shared {
		share = models.InlineKeyboardButton{Text: c.UnshareCardButton(), CallbackData: actionUnshareCard + ":" + arg, Style: styleDanger}
	}
	rows := [][]models.InlineKeyboardButton{{share}}
	// An Attached Library's file lies out of the bot's reach.
	if !card.Track.Attached() {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: c.SendOwnFileButton(), CallbackData: actionSendOwnFile + ":" + arg},
		})
	}
	return window.View{Text: c.TrackCard(card), Rows: append(rows, backRow(ctx, back))}
}

func (s *ShareScreen) albumCardView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	trackID, back := parseCardArg(arg)
	card, err := s.ViewAlbumCard.Execute(ctx, trackID)
	if errors.Is(err, library.ErrNotKeptTrack) {
		return s.Telegram.screens.view(ctx, back.screen, back.arg).WithNotice(c.NotOwnTrack())
	}
	if err != nil {
		slog.Error("view_album_card", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{backRow(ctx, back)}}
	}
	var share []models.InlineKeyboardButton
	if !card.AllShared() {
		share = append(share, models.InlineKeyboardButton{Text: c.ShareAlbumButton(), CallbackData: actionShareAlbumCard + ":" + arg, Style: stylePrimary})
	}
	if card.SharedCount() > 0 {
		share = append(share, models.InlineKeyboardButton{Text: c.UnshareAlbumButton(), CallbackData: actionUnshareAlbumCard + ":" + arg, Style: styleDanger})
	}
	rows := [][]models.InlineKeyboardButton{share}
	if card.Linkable {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: c.ListenLinkButton(), CallbackData: actionSendAlbumLink + ":" + arg},
		})
	}
	for n := range card.Tracks {
		track := &card.Tracks[n]
		rows = append(rows, []models.InlineKeyboardButton{
			goButton(c.OwnTrackButton(track), place{screen: screenShareTrack, arg: cardArg(track.ID, "@"+arg)}),
		})
	}
	return window.View{Text: c.AlbumCard(card), Rows: append(rows, backRow(ctx, back))}
}

func (s *ShareScreen) shareCard(ctx context.Context, cb windowCallback, arg string) {
	trackID, _ := parseCardArg(arg)
	result, err := s.ShareTrack.Execute(ctx, trackID)
	s.afterShare(ctx, cb, place{screen: screenShareTrack, arg: arg}, result, err)
}

func (s *ShareScreen) unshareCard(ctx context.Context, cb windowCallback, arg string) {
	trackID, _ := parseCardArg(arg)
	err := s.UnshareTrack.Execute(ctx, trackID)
	s.afterShare(ctx, cb, place{screen: screenShareTrack, arg: arg}, nil, err)
}

func (s *ShareScreen) shareAlbumCard(ctx context.Context, cb windowCallback, arg string) {
	trackID, _ := parseCardArg(arg)
	result, err := s.ShareAlbum.Execute(ctx, trackID)
	s.afterShare(ctx, cb, place{screen: screenShareAlbum, arg: arg}, result, err)
}

func (s *ShareScreen) unshareAlbumCard(ctx context.Context, cb windowCallback, arg string) {
	trackID, _ := parseCardArg(arg)
	err := s.UnshareAlbum.Execute(ctx, trackID)
	s.afterShare(ctx, cb, place{screen: screenShareAlbum, arg: arg}, nil, err)
}

// afterShare takes a nil result for an Unshare.
func (s *ShareScreen) afterShare(ctx context.Context, cb windowCallback, card place, result *share_tracks.ShareResult, err error) {
	c := texts(ctx)
	switch {
	case err != nil:
		s.Telegram.answerCallback(ctx, cb.query.ID, s.Telegram.shareFailure(c, cb.query.Data, err))
		return
	case result == nil:
		s.Telegram.answerCallback(ctx, cb.query.ID, c.Unshared())
	default:
		s.Telegram.answerCallback(ctx, cb.query.ID, c.ShareResult(result))
	}
	s.Telegram.show(ctx, cb.chatID, cb.messageID, card, "")
}

func (s *ShareScreen) sendOwnFile(ctx context.Context, cb windowCallback, arg string) {
	c := texts(ctx)
	trackID, _ := parseCardArg(arg)
	err := s.sendFile(ctx, cb.chatID, trackID)
	switch {
	case err == nil:
		s.Telegram.answerCallback(ctx, cb.query.ID, "")
	case errors.Is(err, library.ErrNotKeptTrack):
		s.Telegram.answerCallback(ctx, cb.query.ID, c.NotOwnTrack())
	case errors.Is(err, trackfile.ErrFileTooLarge):
		s.Telegram.answerCallback(ctx, cb.query.ID, c.FileTooLarge())
	default:
		slog.Error("send_own_file", "error", err)
		s.Telegram.answerCallback(ctx, cb.query.ID, c.TryLater())
	}
}

func (s *ShareScreen) sendFile(ctx context.Context, chatID int64, trackID uint) error {
	track, path, err := s.GetTrackFile.Execute(ctx, trackID)
	if err != nil {
		return err
	}
	return s.Files.SendTo(ctx, chatID, track, path)
}

func (s *ShareScreen) sendAlbumLink(ctx context.Context, cb windowCallback, arg string) {
	c := texts(ctx)
	trackID, _ := parseCardArg(arg)
	link, err := s.GetAlbumListenLink.Execute(ctx, trackID)
	if link == nil {
		s.Telegram.answerCallback(ctx, cb.query.ID, linkRefusal(c, err))
		return
	}
	s.Telegram.answerCallback(ctx, cb.query.ID, c.ListenLinkSent())
	logUnsent(s.Telegram.sendText(ctx, cb.chatID, albumLinkText(c, link, err)))
}

func senderID(ctx context.Context) int64 {
	s, err := senderFrom(ctx)
	if err != nil {
		return 0
	}
	return s.from.ID
}

func (t *Telegram) shareFailure(c i18n.Catalog, data string, err error) string {
	var full *library.QuotaExceededError
	switch {
	case errors.As(err, &full):
		return c.SharedLibraryFull(t.AdminContact)
	case errors.Is(err, library.ErrNotKeptTrack):
		return c.NotOwnTrack()
	case errors.Is(err, library.ErrInboxTrack):
		return c.InboxNotShareable()
	default:
		slog.Error("share_callback", "data", data, "error", err)
		return c.TryLater()
	}
}
