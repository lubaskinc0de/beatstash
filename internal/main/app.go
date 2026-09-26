package app

import (
	"context"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"gorm.io/gorm"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/add_track"
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
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/audio"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/background"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/database"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/disk"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/navidrome"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/secrets"
	tgbot "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/bot"
	tgprovider "github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/zvuk"
)

type App struct {
	bot       *bot.Bot
	workers   *background.IngestWorkers
	scheduler *background.Scheduler
	poller    *tgbot.Poller
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

	ids := &tgbot.IDProvider{Users: users, Clock: cfg.Clock}
	options := tgbot.Options(ids)
	if cfg.BotAPIURL != "" {
		options = append(options, bot.WithServerURL(cfg.BotAPIURL))
	}
	b, err := bot.New(cfg.Token, append(options, opts...)...)
	if err != nil {
		return nil, err
	}

	audioSender := &tgbot.AudioSender{Bot: b, MaxPostSize: cfg.MaxPostSize}
	telegramFiles := &store.Files{DB: db}
	shares := &database.ShareRepository{DB: db}
	takes := &database.TakeRepository{DB: db}
	providerAccountRepo := &database.ProviderAccountRepository{DB: db}
	providerAccounts := &accounts.ProviderTokens{Repo: providerAccountRepo, Box: box}
	providers := providers.NewRegistry(
		&tgprovider.Provider{Bot: b, Files: telegramFiles},
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
			Tx:          txManager,
			Queue:       ingestQueue,
			Providers:   providers,
			Tracks:      tracks,
			Uploads:     uploads,
			Libraries:   libraryRepo,
			Lock:        libraryLock,
			Disk:        fileDisk,
			Tags:        audio.Tags{},
			Remuxer:     audio.FFmpeg{},
			Batches:     batchRepo,
			InFlight:    inFlight,
			MusicDir:    cfg.MusicDir,
			RetryDelays: cfg.IngestRetryDelays,
		},
		InFlight:      inFlight,
		Queue:         ingestQueue,
		SettleBatches: &ingest_track.SettleIngestBatches{Queue: ingestQueue, Batches: batchRepo},
		Disk:          fileDisk,
		Waker:         waker,
		Lanes: []background.Lane{
			{Filter: repositories.JobFilter{Except: paced}, Workers: cfg.IngestWorkers},
			{Filter: repositories.JobFilter{Only: paced, PerUser: cfg.ZvukPerUser}, Workers: cfg.ZvukWorkers},
		},
		PollInterval: cfg.IngestPollInterval,
	}
	enqueueIngest := &add_track.EnqueueIngest{IDs: ids, Queue: ingestQueue, Waker: waker, Clock: time.Now}
	jobMessages := &store.JobMessages{DB: db}
	batchMessages := &store.BatchMessages{DB: db}
	poller := &tgbot.Poller{
		Bot:             b,
		Jobs:            &ingest_track.GetIngestJobs{Queue: ingestQueue},
		JobMessages:     jobMessages,
		Batches:         &ingest_track.GetIngestBatches{Queue: ingestQueue, Batches: batchRepo},
		BatchMessages:   batchMessages,
		InvalidAccounts: &sync_collection.GetInvalidatedProviderAccounts{Accounts: providerAccountRepo},
		AccountNotices:  &store.AccountNotices{DB: db},
		Files:           telegramFiles,
		Sender:          audioSender,
		StorageChatID:   cfg.StorageChatID,
		MusicDir:        cfg.MusicDir,
		Interval:        cfg.TelegramPollInterval,
	}
	invites := &database.InviteRepository{DB: db}
	createInvite := &invite_friend.CreateInvite{IDs: ids, Invites: invites, TTL: cfg.InviteTTL, Clock: cfg.Clock}

	handler := &tgbot.Handler{
		Zvuk:          zvuk.Name,
		EnqueueIngest: enqueueIngest,
		GetNowPlaying: &show_playing.GetNowPlaying{
			IDs:       ids,
			Client:    navidromeClient,
			Repo:      tracks,
			Accounts:  navidromeAccounts,
			Libraries: libs,
		},
		GetRecentlyPlayed: &show_playing.GetRecentlyPlayed{
			IDs:       ids,
			Client:    navidromeClient,
			Repo:      tracks,
			Accounts:  navidromeAccounts,
			Libraries: libs,
		},
		LinkNavidromeAccount: &connect_navidrome.LinkNavidromeAccount{
			IDs:       ids,
			Navidrome: navidromeClient,
			Accounts:  navidromeAccounts,
			Linked:    accountRepo,
			Libraries: navidromeLibraries,
		},
		CreateInvite:   createInvite,
		CheckCanInvite: &invite_friend.CheckCanInvite{IDs: ids},
		AcceptInvite: &join_by_invite.AcceptInvite{
			IDs:                ids,
			Tx:                 txManager,
			Invites:            invites,
			Users:              users,
			Libraries:          libs,
			NavidromeLibraries: navidromeLibraries,
			Clock:              cfg.Clock,
		},
		RegisterAccount: &connect_navidrome.RegisterNavidromeAccount{
			IDs:       ids,
			Navidrome: navidromeClient,
			Accounts:  navidromeAccounts,
			Libraries: navidromeLibraries,
			Admin:     navidromeAdmin,
		},
		ShowShareOptions: &share_tracks.ShowShareOptions{IDs: ids, Tracks: tracks, Shares: shares, Libraries: libs},
		ShareTrack: &share_tracks.ShareTrack{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shares: shares,
			Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
		},
		ShareAlbum: &share_tracks.ShareAlbum{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shares: shares,
			Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
		},
		UnshareTrack: &share_tracks.UnshareTrack{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shares: shares,
			Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir,
		},
		UnshareAlbum: &share_tracks.UnshareAlbum{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shares: shares,
			Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir,
		},
		ViewFeed: &browse_shared.ViewFeed{IDs: ids, Shares: shares},
		TakeTrack: &browse_shared.TakeTrack{
			IDs: ids, Tx: txManager, Lock: libraryLock, Tracks: tracks, Shares: shares, Takes: takes,
			Libraries: libs, Disk: fileDisk, MusicDir: cfg.MusicDir, Clock: cfg.Clock,
		},
		GetTrackAudio:   &browse_shared.GetTrackAudio{IDs: ids, Tracks: tracks, Libraries: libs},
		GetTop:          &view_top.GetTop{IDs: ids, Shares: shares, Takes: takes, Clock: cfg.Clock},
		GetServiceStats: &greet_stranger.GetServiceStats{Users: users, Tracks: tracks, Libraries: libraryRepo},
		ConnectProviderAccount: &connect_provider.ConnectProviderAccount{
			IDs: ids, Tx: txManager, Providers: providers, Accounts: providerAccountRepo, Box: box,
		},
		DisconnectProviderAccount: &connect_provider.DisconnectProviderAccount{IDs: ids, Tx: txManager, Accounts: providerAccountRepo},
		PlanImport:                &import_collection.PlanImport{IDs: ids, Providers: providers, Libraries: libraryRepo, Tracks: tracks},
		StartImport: &import_collection.StartImport{
			IDs:       ids,
			Tx:        txManager,
			Providers: providers,
			Libraries: libraryRepo,
			Tracks:    tracks,
			Accounts:  providerAccountRepo,
			Queue:     ingestQueue,
			BatchRepo: batchRepo,
			Waker:     waker,
		},
		AdminContact:  cfg.AdminContact,
		Dialogs:       &store.Dialogs{DB: db},
		JobMessages:   jobMessages,
		BatchMessages: batchMessages,
		Files:         telegramFiles,
		Sender:        audioSender,
	}
	handler.Register(b)

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
			Tracks:            tracks,
			Navidrome:         navidromeClient,
			NavidromeAccounts: navidromeAccounts,
		},
		Tick: min(cfg.SyncInterval, cfg.MirrorRetryInterval),
	}

	return &App{bot: b, workers: workers, scheduler: scheduler, poller: poller, db: db}, nil
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
