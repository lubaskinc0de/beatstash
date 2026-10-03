package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/add_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/attach_libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/browse_shared"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/listening"
	appnd "github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/quotas"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/greet_stranger"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/invite_friend"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/join_by_invite"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/manage_quotas"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/oversee_service"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/reconcile_libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/resolve_songs"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/search_music"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/send_listen_link"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/start_app"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/sync_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_home"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/audio"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/background"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/disk"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/secrets"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/stall"
	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/poller"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/zvuk"
)

type App struct {
	bot          *bot.Bot
	workers      *background.IngestWorkers
	scheduler    *background.Scheduler
	attacher     *background.Attacher
	songResolver *background.SongResolver
	sweeper      *background.Sweeper
	reconciler   *background.Reconciler
	poller       *poller.Poller
	db           *gorm.DB

	closeOnce sync.Once
	closeErr  error
}

// New wires the whole application into a bot ready to process updates and
// brings the database, music_dir and Navidrome in step on the way.
// Extra options are applied after the configured ones.
func New(ctx context.Context, cfg Config, opts ...bot.Option) (*App, error) {
	db, err := database.New(cfg.DBDSN)
	if err != nil {
		return nil, err
	}

	a, err := build(ctx, cfg, db, opts)
	if err != nil {
		_ = database.Close(db)
		return nil, err
	}
	return a, nil
}

func build(ctx context.Context, cfg Config, db *gorm.DB, opts []bot.Option) (*App, error) {
	if err := database.Migrate(db); err != nil {
		return nil, err
	}
	box, err := secrets.NewBox(cfg.SecretKey)
	if err != nil {
		return nil, err
	}

	txManager := &database.TxManager{DB: db}
	tracks := &database.TrackRepository{DB: db}
	users := &database.UserRepository{DB: db}
	uploads := &database.UploadRepository{DB: db}
	libraryRepo := &database.LibraryRepository{DB: db}
	libraryLock := &database.LibraryLock{DB: db}
	ingestQueue := &database.IngestQueue{DB: db}
	accountRepo := &database.NavidromeAccountRepository{DB: db}
	publicURL := listenLinkURL(cfg.NavidromePublicURL)
	navidromeClient := navidrome.NewClient(cfg.NavidromeURL, publicURL, &database.NavidromeSessionRepository{DB: db})
	navidromeAdmin := appnd.Credentials{Login: cfg.NavidromeUser, Password: cfg.NavidromePassword}
	navidromeAccounts := &accounts.Navidrome{Repo: accountRepo, Box: box}
	fileDisk := &disk.Disk{
		MusicDir:   cfg.MusicDir,
		Scratch:    filepath.Join(cfg.MusicDir, disk.ScratchDir),
		ScratchTTL: cfg.ScratchTTL,
	}
	fileDisk.SweepScratch()
	folders := library.SystemFolders{Root: cfg.NavidromeMusicDir, Reserved: []string{disk.ScratchDir}}
	quotaSettings := &database.QuotaSettingsRepository{DB: db}
	libraryQuotas := &quotas.Quotas{Repo: quotaSettings, Libraries: libraryRepo, Tracks: tracks, Disk: fileDisk, Config: cfg.Quotas}

	attached := &libraries.Attached{
		Repo:      libraryRepo,
		Accounts:  accountRepo,
		Navidrome: &navidrome.AccountCache{Client: navidromeClient, TTL: cfg.NavidromeAccessTTL, Clock: cfg.Clock},
		Admin:     navidromeAdmin,
	}
	libs := &libraries.Libraries{Repo: libraryRepo, Attached: attached, Disk: fileDisk, MusicDir: cfg.MusicDir}
	navidromeLibraries := &libraries.Navidrome{
		Libraries: libs,
		Navidrome: navidromeClient,
		Admin:     navidromeAdmin,
		Folders:   folders,
	}
	startApp := &start_app.StartApp{
		Admins:             cfg.Admins,
		Users:              users,
		Accounts:           accountRepo,
		Libraries:          libs,
		NavidromeLibraries: navidromeLibraries,
		Clock:              cfg.Clock,
	}
	if err := startApp.Execute(ctx); err != nil {
		return nil, err
	}
	attachLibraries := &attach_libraries.AttachLibraries{
		Tx:        txManager,
		Libraries: libraryRepo,
		Lock:      libraryLock,
		Tracks:    tracks,
		Navidrome: navidromeClient,
		Admin:     navidromeAdmin,
		Folders:   folders,
	}
	attacher := &background.Attacher{Attach: attachLibraries, Interval: cfg.AttachInterval}
	attacher.Once(ctx)
	// Before the Ingest workers: a file a crashed Ingest left in a Library
	// becomes a Track, so the retried job finds it a Duplicate.
	reconciler := &background.Reconciler{
		Reconcile: &reconcile_libraries.ReconcileLibraries{
			Tx: txManager, Libraries: libraryRepo, Lock: libraryLock, Tracks: tracks,
			Disk: fileDisk, Tags: audio.Tags{}, MusicDir: cfg.MusicDir,
		},
		Interval: cfg.ReconcileInterval,
	}
	reconciler.Once(ctx)

	telegramIDs := &tgbot.IDProvider{Users: users, Clock: cfg.Clock}
	ids := &common.Visitors{Channel: telegramIDs, Users: users, Clock: cfg.Clock}
	telegramUsers := &tgbot.Users{DB: db}
	texts, err := i18n.Load(i18n.Options{
		Dir:     cfg.TranslationsDir,
		Default: i18n.Language(cfg.DefaultLanguage),
		Brand:   i18n.Brand{Service: cfg.ServiceName, NavidromeURL: cfg.NavidromePublicURL},
	})
	if err != nil {
		return nil, err
	}
	windows := &window.Windows{DB: db, LeaseTTL: cfg.TelegramLeaseTTL}
	options := tgbot.Options(ids, telegramUsers, windows, texts)
	if cfg.BotAPIURL != "" {
		options = append(options, bot.WithServerURL(cfg.BotAPIURL))
	}
	b, err := bot.New(cfg.Token, append(options, opts...)...)
	if err != nil {
		return nil, err
	}
	windows.Bot = b
	me, err := b.GetMe(ctx)
	if err != nil {
		return nil, err
	}

	audioSender := &trackfile.AudioSender{Bot: b, MaxPostSize: cfg.MaxPostSize}
	telegramFiles := &trackfile.Files{DB: db, Sender: audioSender, LeaseTTL: cfg.TelegramLeaseTTL}
	sharedTracks := &database.SharedTrackRepository{DB: db}
	takes := &database.TakeRepository{DB: db}
	providerAccountRepo := &database.ProviderAccountRepository{DB: db}
	providerAccounts := &accounts.ProviderTokens{Repo: providerAccountRepo, Box: box}
	watch := stall.Watch{Timeout: cfg.StallTimeout, AfterFunc: cfg.AfterFunc}
	providers := providers.NewRegistry(
		&tgprovider.Provider{Bot: b, Files: telegramFiles, DB: db, Stall: watch},
		&zvuk.Provider{
			Client: zvuk.NewClient(cfg.ZvukURL, watch),
			Tokens: providerAccounts,
			Pacer:  &zvuk.Pacer{Min: cfg.ZvukPauseMin, Max: cfg.ZvukPauseMax, PerUser: cfg.ZvukPerUser},
		},
	)
	batchRepo := &database.IngestBatchRepository{DB: db}
	waker := &background.Waker{}
	// Zvuk gets workers of its own: its downloads wait out pauses that
	// must not hold up files sent from Telegram.
	paced := []provider.ProviderName{zvuk.Name}
	inFlight := &ingest_track.InFlight{}
	workers := &background.IngestWorkers{
		ProcessIngestJob: &ingest_track.ProcessIngestJob{
			Tx:        txManager,
			Queue:     ingestQueue,
			Providers: providers,
			Tracks:    tracks,
			Uploads:   uploads,
			Libraries: libs,
			Lock:      libraryLock,
			Quotas:    libraryQuotas,
			Disk:      fileDisk,
			Tags:      audio.Tags{},
			Remuxer:   audio.FFmpeg{},
			Batches:   batchRepo,
			InFlight:  inFlight,
			MusicDir:  cfg.MusicDir,
			Retry:     ingest.RetryPolicy{Delays: cfg.IngestRetryDelays},
		},
		InFlight:      inFlight,
		Queue:         ingestQueue,
		SettleBatches: &ingest_track.SettleIngestBatches{Tx: txManager, Queue: ingestQueue, Batches: batchRepo},
		Waker:         waker,
		Lanes: []background.Lane{
			{Filter: repositories.JobFilter{Except: paced}, Workers: cfg.IngestWorkers},
			{Filter: repositories.JobFilter{Only: paced, PerUser: cfg.ZvukPerUser}, Workers: cfg.ZvukWorkers},
		},
		PollInterval: cfg.IngestPollInterval,
	}
	enqueueIngest := &add_track.EnqueueIngest{IDs: ids, Queue: ingestQueue, Waker: waker, Clock: time.Now}
	invites := &database.InviteRepository{DB: db}
	createInvite := &invite_friend.CreateInvite{IDs: ids, Invites: invites, TTL: cfg.InviteTTL, Clock: cfg.Clock}

	listenLinkRepo := &database.ListenLinkRepository{DB: db}
	listenLinks := &listening.ListenLinks{Accounts: navidromeAccounts, On: publicURL != ""}
	getTrackListenLink := &send_listen_link.GetTrackListenLink{
		IDs: ids, Tracks: tracks, Links: listenLinkRepo,
		Libraries: libs, ListenLinks: listenLinks,
		Navidrome: navidromeClient, Admin: navidromeAdmin,
		TTL: cfg.ListenLinkTTL, Downloadable: cfg.ListenLinkDownloadable, Clock: cfg.Clock,
	}
	getAlbumListenLink := &send_listen_link.GetAlbumListenLink{
		IDs: ids, Tracks: tracks, Links: listenLinkRepo,
		Libraries: libs, ListenLinks: listenLinks,
		Navidrome: navidromeClient, Admin: navidromeAdmin,
		TTL: cfg.ListenLinkTTL, Downloadable: cfg.ListenLinkDownloadable, Clock: cfg.Clock,
	}
	getTrackFile := &show_playing.GetTrackFile{IDs: ids, Tracks: tracks, Libraries: libs}
	tg := &tgbot.Telegram{Bot: b, BotName: me.Username, Windows: windows, Texts: texts, AdminContact: cfg.AdminContact}
	viewFeed := &browse_shared.ViewFeed{IDs: ids, Shared: sharedTracks, Tracks: tracks, Libraries: libs}
	getTop := &view_top.GetTop{IDs: ids, Shared: sharedTracks, Takes: takes, Clock: cfg.Clock}
	home := &tgbot.Home{
		Telegram:     tg,
		IDs:          ids,
		Users:        telegramUsers,
		GetHome:      &view_home.GetHome{IDs: ids, Quotas: libraryQuotas},
		CreateInvite: createInvite,
		AcceptInvite: &join_by_invite.AcceptInvite{
			IDs:                ids,
			Tx:                 txManager,
			Invites:            invites,
			Users:              users,
			Libraries:          libs,
			NavidromeLibraries: navidromeLibraries,
			Clock:              cfg.Clock,
		},
		GetServiceStats:  &greet_stranger.GetServiceStats{Users: users, Tracks: tracks, Libraries: libraryRepo},
		CheckListenLinks: &send_listen_link.CheckListenLinks{IDs: ids, ListenLinks: listenLinks},
	}
	feed := &tgbot.Feed{
		Telegram: tg,
		ViewFeed: viewFeed,
		ViewSharedTrack: &browse_shared.ViewSharedTrack{
			IDs: ids, Shared: sharedTracks, Tracks: tracks, Libraries: libs,
		},
		GetTop: getTop,
		TakeTrack: &browse_shared.TakeTrack{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks, Takes: takes,
			Libraries: libs, Quotas: libraryQuotas, Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
		},
		GetTrackAudio: &browse_shared.GetTrackAudio{IDs: ids, Shared: sharedTracks, Libraries: libs},
		Files:         telegramFiles,
	}
	imports := &tgbot.Imports{
		Telegram:          tg,
		ListImportSources: &import_collection.ListImportSources{IDs: ids, Providers: providers, Accounts: providerAccountRepo},
		ConnectProviderAccount: &connect_provider.ConnectProviderAccount{
			IDs: ids, Tx: txManager, Providers: providers, Accounts: providerAccountRepo, Box: box,
		},
		DisconnectProviderAccount: &connect_provider.DisconnectProviderAccount{IDs: ids, Tx: txManager, Accounts: providerAccountRepo},
		PlanImport: &import_collection.PlanImport{
			IDs: ids, Providers: providers, Libraries: libs, Tracks: tracks, Quotas: libraryQuotas,
		},
		StartImport: &import_collection.StartImport{
			IDs:       ids,
			Tx:        txManager,
			Providers: providers,
			Libraries: libs,
			Tracks:    tracks,
			Accounts:  providerAccountRepo,
			Queue:     ingestQueue,
			Batches:   batchRepo,
			Quotas:    libraryQuotas,
			Waker:     waker,
			Clock:     cfg.Clock,
		},
		GetRunningImports: &import_collection.GetRunningImports{IDs: ids, Queue: ingestQueue, Batches: batchRepo},
		Followed:          &poller.FollowedBatches{DB: db},
	}
	navidromeScreens := &tgbot.Navidrome{
		Telegram:            tg,
		GetNavidromeAccount: &connect_navidrome.GetNavidromeAccount{IDs: ids, Accounts: accountRepo},
		LinkNavidromeAccount: &connect_navidrome.LinkNavidromeAccount{
			IDs:                ids,
			Navidrome:          navidromeClient,
			Accounts:           navidromeAccounts,
			NavidromeLibraries: navidromeLibraries,
			Libraries:          libraryRepo,
			Tracks:             tracks,
		},
		RegisterAccount: &connect_navidrome.RegisterNavidromeAccount{
			IDs:                ids,
			Navidrome:          navidromeClient,
			Accounts:           navidromeAccounts,
			NavidromeLibraries: navidromeLibraries,
			Admin:              navidromeAdmin,
		},
	}
	shareTrack := &share_tracks.ShareTrack{
		IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks,
		Libraries: libs, Quotas: libraryQuotas, Navidrome: navidromeClient, Admin: navidromeAdmin,
		Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
	}
	shareAlbum := &share_tracks.ShareAlbum{
		IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks,
		Libraries: libs, Quotas: libraryQuotas, Navidrome: navidromeClient, Admin: navidromeAdmin,
		Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
	}
	unshareTrack := &share_tracks.UnshareTrack{
		IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks,
		Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir,
	}
	unshareAlbum := &share_tracks.UnshareAlbum{
		IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks,
		Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir,
	}
	inline := &tgbot.Inline{
		Telegram: tg,
		GetNowPlaying: &show_playing.GetNowPlaying{
			IDs:         ids,
			Client:      navidromeClient,
			Tracks:      tracks,
			Accounts:    navidromeAccounts,
			Libraries:   libs,
			ListenLinks: listenLinks,
		},
		GetRecentlyPlayed: &show_playing.GetRecentlyPlayed{
			IDs:         ids,
			Client:      navidromeClient,
			Tracks:      tracks,
			Accounts:    navidromeAccounts,
			Libraries:   libs,
			ListenLinks: listenLinks,
		},
		GetTrackFile: getTrackFile,
		ViewFeed:     viewFeed,
		GetTop:       getTop,
		SearchMusic: &search_music.SearchMusic{
			IDs: ids, Tracks: tracks, Libraries: libs, ListenLinks: listenLinks,
		},
		GetTrackListenLink: getTrackListenLink,
		GetAlbumListenLink: getAlbumListenLink,
		Files:              telegramFiles,
		StorageChatID:      cfg.StorageChatID,
	}
	telegramUploads := &tgbot.Uploads{
		Telegram:      tg,
		EnqueueIngest: enqueueIngest,
		JobMessages:   &poller.JobMessages{DB: db},
	}
	getUserCard := &oversee_service.GetUserCard{IDs: ids, Users: users, Libraries: libraryRepo, Tracks: tracks, Quotas: libraryQuotas}
	quotaScreens := &tgbot.Quotas{
		Telegram:        tg,
		GetServerQuotas: &manage_quotas.GetServerQuotas{IDs: ids, Libraries: libraryRepo, Quotas: libraryQuotas},
		SetDefaultQuota: &manage_quotas.SetDefaultQuota{IDs: ids, Settings: quotaSettings},
		SetSharedQuota:  &manage_quotas.SetSharedQuota{IDs: ids, Settings: quotaSettings},
		SetUserQuota:    &manage_quotas.SetUserQuota{IDs: ids, Libraries: libraryRepo},
		GetUserCard:     getUserCard,
	}
	adminScreens := &tgbot.Admin{
		Telegram: tg,
		GetOverview: &oversee_service.GetOverview{
			IDs: ids, Users: users, Strangers: telegramUsers, Libraries: libraryRepo, Tracks: tracks,
			Takes: takes, Jobs: ingestQueue, Quotas: libraryQuotas, Clock: cfg.Clock,
		},
		ListUsers:   &oversee_service.ListUsers{IDs: ids, Users: users},
		GetUserCard: getUserCard,
		Clock:       cfg.Clock,
	}
	shareScreen := &tgbot.ShareScreen{
		Telegram:        tg,
		Users:           telegramUsers,
		SearchOwnTracks: &search_music.SearchOwnTracks{IDs: ids, Tracks: tracks, Libraries: libs},
		SearchOwnAlbums: &search_music.SearchOwnAlbums{IDs: ids, Tracks: tracks, Libraries: libs},
		ViewTrackCard:   &share_tracks.ViewTrackCard{IDs: ids, Tracks: tracks, Shared: sharedTracks, Libraries: libs},
		ViewAlbumCard: &share_tracks.ViewAlbumCard{
			IDs: ids, Tracks: tracks, Shared: sharedTracks, Libraries: libs, ListenLinks: listenLinks,
		},
		ShareTrack: shareTrack, ShareAlbum: shareAlbum, UnshareTrack: unshareTrack, UnshareAlbum: unshareAlbum,
		GetTrackFile:       getTrackFile,
		GetAlbumListenLink: getAlbumListenLink,
		Files:              telegramFiles,
	}
	handler := &tgbot.Handler{
		Telegram: tg, Home: home, Feed: feed, Imports: imports, Navidrome: navidromeScreens,
		Inline: inline, Uploads: telegramUploads, Admin: adminScreens, Quotas: quotaScreens,
		Share: shareScreen,
	}
	handler.Register()
	telegramPoller := poller.New(poller.Config{
		DB:              db,
		Chats:           &tgbot.Chats{Telegram: tg, Imports: imports, Users: telegramUsers, IDs: telegramIDs},
		Files:           telegramFiles,
		IngestResults:   &add_track.GetIngestResults{IDs: ids, Queue: ingestQueue, Quotas: libraryQuotas},
		ImportResults:   &import_collection.GetImportResults{IDs: ids, Queue: ingestQueue, Batches: batchRepo},
		RejectedTokens:  &connect_provider.ListRejectedTokens{Accounts: providerAccountRepo},
		StorageChatID:   cfg.StorageChatID,
		FillStorageChat: cfg.FillStorageChat,
		MusicDir:        cfg.MusicDir,
		LeaseTTL:        cfg.TelegramLeaseTTL,
		Interval:        cfg.TelegramPollInterval,
	})
	workers.Processed = telegramPoller.Wake
	if err := tgbot.Describe(ctx, b, texts); err != nil {
		slog.Error("describe_bot", "error", err)
	}

	scheduler := &background.Scheduler{
		Provider: zvuk.Name,
		Sync: &sync_collection.SyncCollection{
			Tx:        txManager,
			Accounts:  providerAccountRepo,
			Providers: providers,
			Queue:     ingestQueue,
			Batches:   batchRepo,
			Waker:     waker,
			Interval:  cfg.SyncInterval,
			Clock:     cfg.Clock,

			Libraries:         libs,
			Tracks:            tracks,
			Quotas:            libraryQuotas,
			Navidrome:         navidromeClient,
			NavidromeAccounts: navidromeAccounts,
		},
		Tick: min(cfg.SyncInterval, cfg.MirrorRetryInterval),
	}

	return &App{
		bot: b, workers: workers, scheduler: scheduler, attacher: attacher,
		songResolver: &background.SongResolver{
			Resolve: &resolve_songs.ResolveSongs{
				Libraries: libraryRepo, Tracks: tracks, Navidrome: navidromeClient, Admin: navidromeAdmin,
			},
			Interval: cfg.SongInterval,
		}, sweeper: &background.Sweeper{Disk: fileDisk, Interval: cfg.ScratchTTL},
		reconciler: reconciler,
		poller:     telegramPoller, db: db,
	}, nil
}

func (a *App) Bot() *bot.Bot {
	return a.bot
}

func (a *App) Run(ctx context.Context) {
	stopped := a.StartWorkers(ctx)
	a.bot.Start(ctx)
	<-stopped
}

func (a *App) StartWorkers(ctx context.Context) <-chan struct{} {
	workers := a.workers.Start(ctx)
	var wg sync.WaitGroup
	wg.Go(func() { a.scheduler.Run(ctx) })
	wg.Go(func() { a.attacher.Run(ctx) })
	wg.Go(func() { a.songResolver.Run(ctx) })
	wg.Go(func() { a.sweeper.Run(ctx) })
	wg.Go(func() { a.reconciler.Run(ctx) })
	wg.Go(func() { a.poller.Run(ctx) })
	stopped := make(chan struct{})
	go func() {
		wg.Wait()
		<-workers
		close(stopped)
	}()
	return stopped
}

// WaitIngest blocks until the queued jobs are done and the users have
// heard of them.
func (a *App) WaitIngest(ctx context.Context) error {
	if err := a.workers.WaitIdle(ctx); err != nil {
		return err
	}
	return a.poller.WaitIdle(ctx)
}

func (a *App) Close() error {
	a.closeOnce.Do(func() {
		a.closeErr = database.Close(a.db)
	})
	return a.closeErr
}

// WaitReconcile blocks until the Libraries are reconciled once more.
func (a *App) WaitReconcile(ctx context.Context) error {
	return a.reconciler.Wait(ctx)
}

// WaitAttach blocks until the Attached Libraries are taken from Navidrome
// once more.
func (a *App) WaitAttach(ctx context.Context) error {
	return a.attacher.Wait(ctx)
}

func (a *App) ResolveSongs(ctx context.Context) error {
	return a.songResolver.Resolve.Execute(ctx)
}

// Sync runs Sync and Mirror now and waits for them to finish.
func (a *App) Sync(ctx context.Context) error {
	return a.scheduler.Now(ctx)
}

// Poll delivers now what the users should hear of so far.
func (a *App) Poll(ctx context.Context) error {
	return a.poller.WaitRound(ctx)
}
