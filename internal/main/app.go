package app

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/add_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/attach_libraries"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/browse_shared"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/libraries"
	appnd "github.com/lubaskinc0de/navidrome-tg/internal/application/common/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/greet_stranger"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/invite_friend"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/join_by_invite"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/share_tracks"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/show_playing"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/start_app"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/sync_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/view_top"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/audio"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/background"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/disk"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/secrets"
	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/poller"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/trackfile"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/zvuk"
)

type App struct {
	bot       *bot.Bot
	workers   *background.IngestWorkers
	scheduler *background.Scheduler
	attacher  *background.Attacher
	poller    *poller.Poller
	db        *gorm.DB

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
	navidromeClient := navidrome.NewClient(cfg.NavidromeURL, &database.NavidromeSessionRepository{DB: db})
	navidromeAdmin := appnd.Credentials{Login: cfg.NavidromeUser, Password: cfg.NavidromePassword}
	navidromeAccounts := &accounts.Navidrome{Repo: accountRepo, Box: box}
	fileDisk := &disk.Disk{MusicDir: cfg.MusicDir}

	libs := &libraries.Libraries{Repo: libraryRepo, Disk: fileDisk, MusicDir: cfg.MusicDir}
	navidromeLibraries := &libraries.Navidrome{
		Libraries: libs,
		Navidrome: navidromeClient,
		Admin:     navidromeAdmin,
		MusicDir:  cfg.NavidromeMusicDir,
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
		MusicDir:  cfg.NavidromeMusicDir,
	}
	attacher := &background.Attacher{Attach: attachLibraries, Interval: cfg.AttachInterval}
	attacher.Once(ctx)
	attached := &libraries.Attached{
		Repo:      libraryRepo,
		Accounts:  accountRepo,
		Navidrome: navidromeClient,
		Admin:     navidromeAdmin,
	}

	ids := &tgbot.IDProvider{Users: users, Clock: cfg.Clock}
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
	providers := providers.NewRegistry(
		&tgprovider.Provider{Bot: b, Files: telegramFiles, DB: db},
		&zvuk.Provider{
			Client: zvuk.NewClient(cfg.ZvukURL),
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
			Libraries: libraryRepo,
			Attached:  attached,
			Lock:      libraryLock,
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
		Disk:          fileDisk,
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

	shareDeps := share_tracks.ShareDeps{
		IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks,
		Libraries: libs, Attached: attached, Navidrome: navidromeClient, Admin: navidromeAdmin,
		Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
	}
	tg := &tgbot.Telegram{Bot: b, BotName: me.Username, Windows: windows, Texts: texts}
	viewFeed := &browse_shared.ViewFeed{IDs: ids, Shared: sharedTracks, Tracks: tracks, Libraries: libs, Attached: attached}
	getTop := &view_top.GetTop{IDs: ids, Shared: sharedTracks, Takes: takes, Clock: cfg.Clock}
	home := &tgbot.Home{
		Telegram:       tg,
		IDs:            ids,
		Users:          telegramUsers,
		CheckCanInvite: &invite_friend.CheckCanInvite{IDs: ids},
		CreateInvite:   createInvite,
		AcceptInvite: &join_by_invite.AcceptInvite{
			IDs:                ids,
			Tx:                 txManager,
			Invites:            invites,
			Users:              users,
			Libraries:          libs,
			NavidromeLibraries: navidromeLibraries,
			Clock:              cfg.Clock,
		},
		GetServiceStats: &greet_stranger.GetServiceStats{Users: users, Tracks: tracks, Libraries: libraryRepo},
		AdminContact:    cfg.AdminContact,
	}
	feed := &tgbot.Feed{
		Telegram: tg,
		ViewFeed: viewFeed,
		GetTop:   getTop,
		TakeTrack: &browse_shared.TakeTrack{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks, Takes: takes,
			Libraries: libs, Attached: attached, Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
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
			IDs: ids, Providers: providers, Libraries: libraryRepo, Attached: attached, Tracks: tracks,
		},
		StartImport: &import_collection.StartImport{
			IDs:       ids,
			Tx:        txManager,
			Providers: providers,
			Libraries: libraryRepo,
			Attached:  attached,
			Tracks:    tracks,
			Accounts:  providerAccountRepo,
			Queue:     ingestQueue,
			BatchRepo: batchRepo,
			Waker:     waker,
		},
		GetRunningImports: &import_collection.GetRunningImports{IDs: ids, Queue: ingestQueue, Batches: batchRepo},
		Followed:          &poller.FollowedBatches{DB: db},
	}
	navidromeScreens := &tgbot.Navidrome{
		Telegram:            tg,
		GetNavidromeAccount: &connect_navidrome.GetNavidromeAccount{IDs: ids, Accounts: accountRepo},
		LinkNavidromeAccount: &connect_navidrome.LinkNavidromeAccount{
			IDs:       ids,
			Navidrome: navidromeClient,
			Accounts:  navidromeAccounts,
			Linked:    accountRepo,
			Libraries: navidromeLibraries,
			Attached:  libraryRepo,
			Tracks:    tracks,
		},
		RegisterAccount: &connect_navidrome.RegisterNavidromeAccount{
			IDs:       ids,
			Navidrome: navidromeClient,
			Accounts:  navidromeAccounts,
			Libraries: navidromeLibraries,
			Admin:     navidromeAdmin,
		},
	}
	sharing := &tgbot.Sharing{
		Telegram:         tg,
		ShowShareOptions: &share_tracks.ShowShareOptions{IDs: ids, Tracks: tracks, Shared: sharedTracks, Libraries: libs},
		ShareTrack:       &share_tracks.ShareTrack{ShareDeps: shareDeps},
		ShareAlbum:       &share_tracks.ShareAlbum{ShareDeps: shareDeps},
		UnshareTrack: &share_tracks.UnshareTrack{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks,
			Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir,
		},
		UnshareAlbum: &share_tracks.UnshareAlbum{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shared: sharedTracks,
			Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir,
		},
	}
	inline := &tgbot.Inline{
		Telegram: tg,
		GetNowPlaying: &show_playing.GetNowPlaying{
			IDs:       ids,
			Client:    navidromeClient,
			Repo:      tracks,
			Accounts:  navidromeAccounts,
			Libraries: libs,
			Attached:  attached,
		},
		GetRecentlyPlayed: &show_playing.GetRecentlyPlayed{
			IDs:       ids,
			Client:    navidromeClient,
			Repo:      tracks,
			Accounts:  navidromeAccounts,
			Libraries: libs,
			Attached:  attached,
		},
		GetTrackFile:  &show_playing.GetTrackFile{IDs: ids, Repo: tracks, Libraries: libs, Attached: attached},
		ViewFeed:      viewFeed,
		GetTop:        getTop,
		Files:         telegramFiles,
		StorageChatID: cfg.StorageChatID,
	}
	telegramUploads := &tgbot.Uploads{
		Telegram:      tg,
		EnqueueIngest: enqueueIngest,
		JobMessages:   &poller.JobMessages{DB: db},
	}
	handler := &tgbot.Handler{
		Telegram: tg, Home: home, Feed: feed, Imports: imports, Navidrome: navidromeScreens,
		Sharing: sharing, Inline: inline, Uploads: telegramUploads,
	}
	handler.Register()
	telegramPoller := poller.New(poller.Config{
		DB:              db,
		Chats:           &tgbot.Chats{Telegram: tg, Imports: imports, Users: telegramUsers, IDs: ids},
		Files:           telegramFiles,
		IngestResults:   &add_track.GetIngestResults{IDs: ids, Queue: ingestQueue},
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
			BatchRepo: batchRepo,
			Waker:     waker,
			Interval:  cfg.SyncInterval,

			Libraries:         libraryRepo,
			Attached:          attached,
			Tracks:            tracks,
			Navidrome:         navidromeClient,
			NavidromeAccounts: navidromeAccounts,
		},
		Tick: min(cfg.SyncInterval, cfg.MirrorRetryInterval),
	}

	return &App{bot: b, workers: workers, scheduler: scheduler, attacher: attacher, poller: telegramPoller, db: db}, nil
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
