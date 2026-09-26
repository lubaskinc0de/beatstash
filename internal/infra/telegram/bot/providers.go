package bot

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

func (h *Handler) sourcesView(ctx context.Context) view {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenHome})
	sources, err := h.ListImportSources.Execute(ctx)
	if err != nil {
		slog.Error("list_import_sources", "error", err)
		return view{text: c.TryLater(), rows: [][]models.InlineKeyboardButton{back}}
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(sources)+2)
	for _, source := range sources {
		rows = append(rows, []models.InlineKeyboardButton{
			goButton(c.ProviderButton(source.Provider), place{screen: screenProvider, arg: string(source.Provider)}),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{goButton(c.ImportsButton(), place{screen: screenImports})}, back)
	return view{text: c.ImportSources(), rows: rows}
}

func (h *Handler) providerView(ctx context.Context, arg string) view {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	back := backRow(ctx, place{screen: screenSources})
	sources, err := h.ListImportSources.Execute(ctx)
	if err != nil {
		slog.Error("list_import_sources", "error", err)
		return view{text: c.TryLater(), rows: [][]models.InlineKeyboardButton{back}}
	}
	status := import_collection.SourceNotConnected
	for _, source := range sources {
		if source.Provider == name {
			status = source.Status
		}
	}
	actions := []models.InlineKeyboardButton{goButton(c.Connect(), place{screen: screenConnect, arg: arg})}
	if status == import_collection.SourceConnected {
		actions = []models.InlineKeyboardButton{
			goButton(c.ImportCollection(), place{screen: screenPlan, arg: arg}),
			{Text: c.Disconnect(), CallbackData: actionDisconnect + ":" + arg},
		}
	}
	return view{text: c.Provider(name, status), rows: [][]models.InlineKeyboardButton{actions, back}}
}

func (h *Handler) connectView(ctx context.Context, arg string) view {
	c := texts(ctx)
	return view{text: c.TokenPrompt(provider.ProviderName(arg)), rows: [][]models.InlineKeyboardButton{
		{goButton(c.Cancel(), place{screen: screenProvider, arg: arg})},
	}}
}

// connectProvider deletes the message at once: it holds a token.
func (h *Handler) connectProvider(ctx context.Context, b *bot.Bot, in windowInput) {
	c := texts(ctx)
	name := provider.ProviderName(in.arg)
	deleteMessage(ctx, b, in.chatID, in.messageID)
	here := place{screen: screenConnect, arg: in.arg}

	err := h.ConnectProviderAccount.Execute(ctx, name, in.text)
	switch {
	case err == nil:
		h.showInWindow(ctx, b, in.chatID, place{screen: screenProvider, arg: in.arg}, c.Connected(name))
	case errors.Is(err, providers.ErrUnauthorized):
		h.showInWindow(ctx, b, in.chatID, here, c.TokenInvalid(name))
	case errors.Is(err, providers.ErrNoSubscription):
		h.showInWindow(ctx, b, in.chatID, here, c.NoSubscription(name))
	default:
		slog.Error("connect_provider", "provider", name, "error", err)
		h.showInWindow(ctx, b, in.chatID, here, c.ProviderDown(name))
	}
}

func (h *Handler) disconnect(ctx context.Context, b *bot.Bot, cb windowCallback, arg string) {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	here := place{screen: screenProvider, arg: arg}
	if err := h.DisconnectProviderAccount.Execute(ctx, name); err != nil {
		slog.Error("disconnect_provider", "provider", name, "error", err)
		answerCallback(ctx, b, cb.query.ID, c.TryLater())
		return
	}
	answerCallback(ctx, b, cb.query.ID, "")
	h.show(ctx, b, cb.messageRef, here, c.Disconnected(name))
}

func (h *Handler) planView(ctx context.Context, arg string) view {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	back := backRow(ctx, place{screen: screenProvider, arg: arg})
	plan, err := h.PlanImport.Execute(ctx, name)
	switch {
	case err == nil:
	case isAccountProblem(err):
		return h.providerView(ctx, arg).withNotice(accountProblem(c, name, err))
	default:
		slog.Error("plan_import", "provider", name, "error", err)
		return h.providerView(ctx, arg).withNotice(c.ProviderDown(name))
	}

	switch {
	case plan.Total == 0:
		return view{text: c.NothingToImport(name), rows: [][]models.InlineKeyboardButton{back}}
	case plan.Missing == 0:
		return view{text: c.AllImported(name, plan.Total), rows: [][]models.InlineKeyboardButton{back}}
	}
	return view{text: c.Plan(name, plan), rows: [][]models.InlineKeyboardButton{
		{{Text: c.StartImport(), CallbackData: actionStartImport + ":" + arg}},
		back,
	}}
}

func (h *Handler) startImport(ctx context.Context, b *bot.Bot, cb windowCallback, arg string) {
	c := texts(ctx)
	name := provider.ProviderName(arg)
	plan, err := h.StartImport.Execute(ctx, name)
	switch {
	case err == nil && plan.Missing == 0:
		answerCallback(ctx, b, cb.query.ID, c.AllInLibrary())
		h.show(ctx, b, cb.messageRef, place{screen: screenPlan, arg: arg}, "")
	case err == nil:
		answerCallback(ctx, b, cb.query.ID, "")
		if err := h.Followed.Follow(ctx, store.FollowedBatch{BatchID: plan.BatchID, ChatID: cb.chatID}); err != nil {
			slog.Error("follow_batch", "batch_id", plan.BatchID, "error", err)
		}
		h.show(ctx, b, cb.messageRef, place{screen: screenImports}, "")
	case errors.Is(err, accounts.ErrBatchRunning):
		answerCallback(ctx, b, cb.query.ID, c.ImportRunning())
	case isAccountProblem(err):
		answerCallback(ctx, b, cb.query.ID, "")
		h.show(ctx, b, cb.messageRef, place{screen: screenProvider, arg: arg}, accountProblem(c, name, err))
	default:
		slog.Error("start_import", "provider", name, "error", err)
		answerCallback(ctx, b, cb.query.ID, c.ProviderDown(name))
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

func (h *Handler) importsView(ctx context.Context) view {
	c := texts(ctx)
	back := backRow(ctx, place{screen: screenSources})
	running, err := h.GetRunningImports.Execute(ctx)
	if err != nil {
		slog.Error("get_running_imports", "error", err)
		return view{text: c.TryLater(), rows: [][]models.InlineKeyboardButton{back}}
	}
	return view{text: c.Imports(running), rows: [][]models.InlineKeyboardButton{back}}
}
