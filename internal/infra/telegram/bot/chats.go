package bot

import (
	"context"
	"errors"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/import_collection"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/library"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/i18n"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/poller"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/telegram/window"
)

// Chats is how the Poller writes to the chats.
type Chats struct {
	Telegram *Telegram
	Imports  *Imports
	Users    *Users
	IDs      *IDProvider
}

var _ poller.Chats = (*Chats)(nil)

func (ch *Chats) Recipients(ctx context.Context, chatIDs []int64) (poller.Recipients, error) {
	languages, err := ch.Users.Languages(ctx, chatIDs)
	if err != nil {
		return nil, err
	}
	return recipients{texts: ch.Telegram.Texts, languages: languages}, nil
}

func (ch *Chats) ChatsOf(ctx context.Context, userIDs []uint) (map[uint]int64, error) {
	return ch.IDs.ChatsOf(ctx, userIDs)
}

type recipients struct {
	texts     *i18n.Bundle
	languages map[int64]string
}

// As gives a user who never chose a language the default one.
func (r recipients) As(ctx context.Context, chatID int64) context.Context {
	return withTexts(asRecipient(ctx, chatID), r.texts.For(i18n.Language(r.languages[chatID])))
}

func (ch *Chats) AnswerIngest(ctx context.Context, m poller.JobMessage, job *ingest.IngestJob, usage library.Usage) error {
	return worthRetry(ch.answerIngest(ctx, m, job, usage))
}

func (ch *Chats) answerIngest(ctx context.Context, m poller.JobMessage, job *ingest.IngestJob, usage library.Usage) error {
	c := texts(ctx)
	msg := messageRef{chatID: m.ChatID, messageID: m.MessageID}
	switch {
	case job.FailureReason == ingest.ReasonQuotaExceeded:
		return ch.Telegram.reject(ctx, msg, c.NoRoom(usage, ch.Telegram.AdminContact))
	case job.Failed():
		return ch.Telegram.reject(ctx, msg, c.UploadFailed(job.FailureReason))
	}

	if err := ch.Telegram.setReaction(ctx, msg, "👍"); err != nil {
		return err
	}
	switch job.Outcome {
	case library.StoredInInbox:
		return ch.Telegram.replyTo(ctx, msg, c.StoredInInbox())
	case library.AlreadyExists:
		return ch.Telegram.replyTo(ctx, msg, c.AlreadyExists())
	case library.Stored, library.Replaced:
		// The reaction alone reports success.
	}
	return nil
}

func (ch *Chats) SummarizeImport(ctx context.Context, chatID int64, result *import_collection.ImportResult) error {
	return worthRetry(ch.Telegram.sendText(ctx, chatID, texts(ctx).ImportSummary(result, ch.Telegram.AdminContact)))
}

func (ch *Chats) NoticeRejectedToken(ctx context.Context, chatID int64, providerName provider.ProviderName) {
	c := texts(ctx)
	logUnsent(ch.Telegram.sendKeyboard(ctx, chatID, c.TokenRejected(providerName), &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{
			goButton(c.Reconnect(), place{screen: screenProvider, arg: string(providerName)}),
		}},
	}))
}

// worthRetry drops the errors a later try would get too: the user blocked
// the bot or deleted the message.
func worthRetry(err error) error {
	if errors.Is(err, bot.ErrorForbidden) || errors.Is(err, bot.ErrorBadRequest) || errors.Is(err, bot.ErrorNotFound) {
		logUnsent(err)
		return nil
	}
	return err
}

// RefreshImports redraws the windows on the Imports whose text changed.
func (ch *Chats) RefreshImports(ctx context.Context) error {
	chatIDs, err := ch.Telegram.Windows.On(ctx, string(screenImports))
	if err != nil || len(chatIDs) == 0 {
		return err
	}
	recipients, err := ch.Recipients(ctx, chatIDs)
	if err != nil {
		return err
	}
	views := make(map[int64]window.View, len(chatIDs))
	for _, chatID := range chatIDs {
		views[chatID] = ch.Imports.importsView(recipients.As(ctx, chatID))
	}
	return ch.Telegram.Windows.Refresh(ctx, string(screenImports), views)
}
