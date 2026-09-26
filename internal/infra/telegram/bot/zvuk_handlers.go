package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/accounts"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/providers"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/common/repositories"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/store"
)

const zvukUsage = "Чтобы подключить Звук, отправьте: <code>/zvuk токен</code>\n\n" + zvukTokenHowTo

const zvukTokenHowTo = "Где взять токен: войдите в zvuk.com в браузере, откройте https://zvuk.com/api/tiny/profile " +
	"и скопируйте значение поля <code>token</code>. Нужна подписка"

const zvukConnectHint = "🔌 Подключите Звук: <code>/zvuk токен</code>"

const actionStartImport = "zi"

func (h *Handler) handleZvuk(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	_, token, _ := command(update)
	if token == "" {
		sendText(ctx, b, chatID, zvukUsage)
		return
	}

	deleteMessage(ctx, b, chatID, update.Message.ID)

	err := h.ConnectProviderAccount.Execute(ctx, h.Zvuk, token)
	switch {
	case err == nil:
		sendText(ctx, b, chatID, "✅ Звук подключён. Сообщение с токеном удалено\n\n"+
			"<code>/zvuk_import</code> — перенести лайки, альбомы и плейлисты в вашу библиотеку")
	case errors.Is(err, providers.ErrUnauthorized):
		sendText(ctx, b, chatID, "❌ Звук не принял токен. Сообщение с токеном удалено, проверьте его и отправьте ещё раз")
	case errors.Is(err, providers.ErrNoSubscription):
		sendText(ctx, b, chatID, "❌ У этого аккаунта Звука нет подписки, а без неё треки не скачать. Сообщение с токеном удалено")
	default:
		slog.Error("connect_zvuk", "error", err)
		sendText(ctx, b, chatID, "⚠️ Звук недоступен, не получилось проверить токен. Сообщение с токеном удалено, попробуйте позже")
	}
}

func (h *Handler) handleZvukOff(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	if err := h.DisconnectProviderAccount.Execute(ctx, h.Zvuk); err != nil {
		slog.Error("disconnect_zvuk", "error", err)
		sendText(ctx, b, chatID, "⚠️ Не получилось отключить Звук, попробуйте позже")
		return
	}
	sendText(ctx, b, chatID, "🔌 Звук отключён: токен удалён, коллекция больше не синхронизируется. Загруженные треки остались в библиотеке")
}

func (h *Handler) handleZvukImport(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	plan, err := h.PlanImport.Execute(ctx, h.Zvuk)
	switch {
	case err == nil:
	case isNoZvukAccount(err):
		sendText(ctx, b, chatID, zvukAccountProblem(err))
		return
	default:
		slog.Error("plan_zvuk_import", "error", err)
		sendText(ctx, b, chatID, "⚠️ Звук недоступен, попробуйте позже")
		return
	}

	switch {
	case plan.Total == 0:
		sendText(ctx, b, chatID, "💤 В коллекции Звука нет ни треков, ни альбомов, ни плейлистов")
		return
	case plan.Missing == 0:
		sendText(ctx, b, chatID, fmt.Sprintf("✅ Вся коллекция Звука (%s) уже в вашей библиотеке", tracksCount(plan.Total)))
		return
	}
	size := fmt.Sprintf("%s, примерно %s", tracksCount(plan.Missing), formatBytes(plan.MissingBytes))
	if plan.Missing < plan.Total {
		size = fmt.Sprintf("%s, в библиотеке ещё нет %s", tracksCount(plan.Total), size)
	}
	sendKeyboard(ctx, b, chatID,
		"🎧 В коллекции Звука "+size+". Лайки, сохранённые альбомы и плейлисты попадут в вашу личную библиотеку, "+
			"лайки станут звёздами, а плейлисты — плейлистами Navidrome. Треки качаются с паузами, это займёт время",
		&models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "▶️ Начать Import", CallbackData: callbackData(actionStartImport, 0)},
		}}},
	)
}

func (h *Handler) handleStartImport(ctx context.Context, b *bot.Bot, query *models.CallbackQuery, _ uint) {
	plan, err := h.StartImport.Execute(ctx, h.Zvuk)
	switch {
	case err == nil && plan.Missing == 0:
		answerCallback(ctx, b, query.ID, "Вся коллекция уже в библиотеке")
		editKeyboard(ctx, b, query, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}})
	case err == nil:
		answerCallback(ctx, b, query.ID, "")
		editKeyboard(ctx, b, query, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}})
		h.showProgress(ctx, b, query.From.ID, plan)
	case errors.Is(err, accounts.ErrBatchRunning):
		answerCallback(ctx, b, query.ID, "Import уже идёт")
	case isNoZvukAccount(err):
		answerCallback(ctx, b, query.ID, "")
		sendText(ctx, b, query.From.ID, zvukAccountProblem(err))
	default:
		slog.Error("start_zvuk_import", "error", err)
		answerCallback(ctx, b, query.ID, "⚠️ Звук недоступен, попробуйте позже")
	}
}

func isNoZvukAccount(err error) bool {
	return errors.Is(err, repositories.ErrProviderAccountNotFound) || errors.Is(err, providers.ErrUnauthorized)
}

func zvukAccountProblem(err error) string {
	if errors.Is(err, providers.ErrUnauthorized) {
		return "🔌 Звук больше не принимает токен. Подключите его заново: <code>/zvuk токен</code>"
	}
	return zvukConnectHint
}

func formatBytes(n int64) string {
	const mb = 1 << 20
	if n < 1<<30 {
		return fmt.Sprintf("%d МБ", max(n/mb, 1))
	}
	return strings.Replace(fmt.Sprintf("%.1f ГБ", float64(n)/(1<<30)), ".", ",", 1)
}

// showProgress sends the message the Poller keeps editing until the Import
// ends.
func (h *Handler) showProgress(ctx context.Context, b *bot.Bot, chatID int64, plan *import_collection.Plan) {
	text := batchText(&ingest_track.BatchState{Batch: ingest.IngestBatch{Provider: h.Zvuk, Total: plan.Missing}})
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML})
	if err != nil {
		slog.Error("send_progress_message", "error", err)
		return
	}
	err = h.BatchMessages.Remember(ctx, store.BatchMessage{BatchID: plan.BatchID, ChatID: chatID, MessageID: msg.ID, Shown: text})
	if err != nil {
		slog.Error("remember_batch_message", "batch_id", plan.BatchID, "error", err)
	}
}
