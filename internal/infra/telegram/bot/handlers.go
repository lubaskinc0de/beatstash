package bot

import (
	"github.com/lubaskinc0de/navidrome-tg/internal/application/add_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/browse_shared"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/greet_stranger"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/invite_friend"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/join_by_invite"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

type Handler struct {
	IDs common.IDProvider

	EnqueueIngest             *add_track.EnqueueIngest
	GetNowPlaying             *show_playing.GetNowPlaying
	GetRecentlyPlayed         *show_playing.GetRecentlyPlayed
	GetTrackFile              *show_playing.GetTrackFile
	LinkNavidromeAccount      *connect_navidrome.LinkNavidromeAccount
	GetNavidromeAccount       *connect_navidrome.GetNavidromeAccount
	CreateInvite              *invite_friend.CreateInvite
	CheckCanInvite            *invite_friend.CheckCanInvite
	AcceptInvite              *join_by_invite.AcceptInvite
	RegisterAccount           *connect_navidrome.RegisterNavidromeAccount
	ShowShareOptions          *share_tracks.ShowShareOptions
	ShareTrack                *share_tracks.ShareTrack
	ShareAlbum                *share_tracks.ShareAlbum
	UnshareTrack              *share_tracks.UnshareTrack
	UnshareAlbum              *share_tracks.UnshareAlbum
	ViewFeed                  *browse_shared.ViewFeed
	TakeTrack                 *browse_shared.TakeTrack
	GetTrackAudio             *browse_shared.GetTrackAudio
	GetTop                    *view_top.GetTop
	GetServiceStats           *greet_stranger.GetServiceStats
	ConnectProviderAccount    *connect_provider.ConnectProviderAccount
	DisconnectProviderAccount *connect_provider.DisconnectProviderAccount
	ListImportSources         *import_collection.ListImportSources
	PlanImport                *import_collection.PlanImport
	StartImport               *import_collection.StartImport
	GetRunningImports         *import_collection.GetRunningImports
	AdminContact              string

	Windows     *store.Windows
	Users       *store.Users
	JobMessages *store.JobMessages
	Followed    *store.FollowedBatches
	Files       *store.Files
	Sender      *AudioSender
	Texts       *i18n.Bundle

	// StorageChatID gets the Tracks without a Telegram file, so inline mode
	// sends them as audio; zero turns it off.
	StorageChatID int64

	names botNames
	chats chatLocks
}
