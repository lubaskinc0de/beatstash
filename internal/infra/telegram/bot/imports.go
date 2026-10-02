package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/connect_provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/poller"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

// Imports connects Providers and runs Imports.
type Imports struct {
	Telegram                  *Telegram
	ListImportSources         *import_collection.ListImportSources
	ConnectProviderAccount    *connect_provider.ConnectProviderAccount
	DisconnectProviderAccount *connect_provider.DisconnectProviderAccount
	PlanImport                *import_collection.PlanImport
	StartImport               *import_collection.StartImport
	GetRunningImports         *import_collection.GetRunningImports
	Followed                  *poller.FollowedBatches
}

func (i *Imports) sourcesView(ctx context.Context) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	sources, err := i.ListImportSources.Execute(ctx)
	if err != nil {
		slog.Error("list_import_sources", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(sources)+2)
	for _, source := range sources {
		rows = append(rows, []models.InlineKeyboardButton{
			goButton(c.ProviderButton(source.Provider), place{screen: screenProvider, arg: string(source.Provider)}),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{goButton(c.ImportsButton(), place{screen: screenImports})}, back)
	return window.View{Text: c.ImportSources(), Rows: rows}
}

func (i *Imports) providerView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	back := backRow(ctx, place{screen: screenSources})
	sources, err := i.ListImportSources.Execute(ctx)
	if err != nil {
		slog.Error("list_import_sources", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	status := import_collection.SourceNotConnected
	for _, source := range sources {
		if source.Provider == name {
			status = source.Status
		}
	}
	actions := []models.InlineKeyboardButton{styled(goButton(c.Connect(), place{screen: screenConnect, arg: arg}), stylePrimary)}
	if status == import_collection.SourceConnected {
		actions = []models.InlineKeyboardButton{
			styled(goButton(c.ImportCollection(), place{screen: screenPlan, arg: arg}), stylePrimary),
			{Text: c.Disconnect(), CallbackData: actionDisconnect + ":" + arg, Style: styleDanger},
		}
	}
	return window.View{Text: c.Provider(name, status), Rows: [][]models.InlineKeyboardButton{actions, back}}
}

func (i *Imports) connectView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	return window.View{Text: c.TokenPrompt(provider.ProviderName(arg)), Rows: [][]models.InlineKeyboardButton{
		{styled(goButton(c.Cancel(), place{screen: screenProvider, arg: arg}), styleDanger)},
	}}
}

func (i *Imports) connectProvider(ctx context.Context, in windowInput) {
	c := texts(ctx)
	name := provider.ProviderName(in.arg)
	here := place{screen: screenConnect, arg: in.arg}

	err := i.ConnectProviderAccount.Execute(ctx, name, in.text)
	switch {
	case err == nil:
		i.Telegram.show(ctx, in.chatID, window.Current, place{screen: screenProvider, arg: in.arg}, c.Connected(name))
	case errors.Is(err, providers.ErrUnauthorized):
		i.Telegram.show(ctx, in.chatID, window.Current, here, c.TokenInvalid(name))
	case errors.Is(err, providers.ErrNoSubscription):
		i.Telegram.show(ctx, in.chatID, window.Current, here, c.NoSubscription(name))
	default:
		slog.Error("connect_provider", "provider", name, "error", err)
		i.Telegram.show(ctx, in.chatID, window.Current, here, c.ProviderDown(name))
	}
}

func (i *Imports) disconnect(ctx context.Context, cb windowCallback, arg string) {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	here := place{screen: screenProvider, arg: arg}
	if err := i.DisconnectProviderAccount.Execute(ctx, name); err != nil {
		slog.Error("disconnect_provider", "provider", name, "error", err)
		i.Telegram.answerCallback(ctx, cb.query.ID, c.TryLater())
		return
	}
	i.Telegram.answerCallback(ctx, cb.query.ID, "")
	i.Telegram.show(ctx, cb.chatID, cb.messageID, here, c.Disconnected(name))
}

func (i *Imports) planView(ctx context.Context, arg string) window.View {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	back := backRow(ctx, place{screen: screenProvider, arg: arg})
	plan, err := i.PlanImport.Execute(ctx, name)
	switch {
	case err == nil:
	case isAccountProblem(err):
		return i.providerView(ctx, arg).WithNotice(accountProblem(c, name, err))
	default:
		slog.Error("plan_import", "provider", name, "error", err)
		return i.providerView(ctx, arg).WithNotice(c.ProviderDown(name))
	}

	switch {
	case plan.Total == 0:
		return window.View{Text: c.NothingToImport(name), Rows: [][]models.InlineKeyboardButton{back}}
	case plan.Missing == 0:
		return window.View{Text: c.AllImported(name, plan.Total), Rows: [][]models.InlineKeyboardButton{back}}
	}
	return window.View{Text: c.Plan(name, plan), Rows: [][]models.InlineKeyboardButton{
		{{Text: c.StartImport(), CallbackData: actionStartImport + ":" + arg, Style: stylePrimary}},
		back,
	}}
}

func (i *Imports) startImport(ctx context.Context, cb windowCallback, arg string) {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	plan, err := i.StartImport.Execute(ctx, name)
	var full *library.QuotaExceededError
	switch {
	case errors.As(err, &full):
		i.Telegram.answerCallback(ctx, cb.query.ID, c.NoRoom(full.Usage, i.Telegram.AdminContact))
	case err == nil && plan.Missing == 0:
		i.Telegram.answerCallback(ctx, cb.query.ID, c.AllInLibrary())
		i.Telegram.show(ctx, cb.chatID, cb.messageID, place{screen: screenPlan, arg: arg}, "")
	case err == nil:
		i.Telegram.answerCallback(ctx, cb.query.ID, "")
		if err := i.Followed.Follow(ctx, poller.FollowedBatch{BatchID: plan.BatchID, ChatID: cb.chatID}); err != nil {
			slog.Error("follow_batch", "batch_id", plan.BatchID, "error", err)
		}
		i.Telegram.show(ctx, cb.chatID, cb.messageID, place{screen: screenImports}, "")
	case errors.Is(err, accounts.ErrBatchRunning):
		i.Telegram.answerCallback(ctx, cb.query.ID, c.ImportRunning())
	case isAccountProblem(err):
		i.Telegram.answerCallback(ctx, cb.query.ID, "")
		i.Telegram.show(ctx, cb.chatID, cb.messageID, place{screen: screenProvider, arg: arg}, accountProblem(c, name, err))
	default:
		slog.Error("start_import", "provider", name, "error", err)
		i.Telegram.answerCallback(ctx, cb.query.ID, c.ProviderDown(name))
	}
}

func isAccountProblem(err error) bool {
	return errors.Is(err, repositories.ErrProviderAccountNotFound) || errors.Is(err, providers.ErrUnauthorized)
}

func accountProblem(c i18n.Catalog, name provider.ProviderName, err error) string {
	if errors.Is(err, providers.ErrUnauthorized) {
		return c.TokenRejected(name)
	}
	return c.NotConnected(name)
}

func (i *Imports) importsView(ctx context.Context) window.View {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenSources})
	running, err := i.GetRunningImports.Execute(ctx)
	if err != nil {
		slog.Error("get_running_imports", "error", err)
		return window.View{Text: c.TryLater(), Rows: [][]models.InlineKeyboardButton{back}}
	}
	return window.View{Text: c.Imports(running), Rows: [][]models.InlineKeyboardButton{back}}
}
