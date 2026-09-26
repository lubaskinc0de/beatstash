package bot

import (
	"fmt"
	"html"
	"strings"

	"github.com/lubaskinc0de/navidrome-tg/internal/application/ingest_track"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
	"github.com/lubaskinc0de/navidrome-tg/internal/domain/provider"
	"github.com/lubaskinc0de/navidrome-tg/internal/infra/zvuk"
)

var importNames = map[provider.ProviderName]string{
	zvuk.Name: "Import из Звука",
}

func importName(batch *ingest.IngestBatch) string {
	if name, ok := importNames[batch.Provider]; ok {
		return name
	}
	return "Import из " + string(batch.Provider)
}

// maxFailedListed keeps a summary within Telegram's 4096 characters.
const maxFailedListed = 40

func batchText(state *ingest_track.BatchState) string {
	if state.Finished() {
		return batchSummary(state)
	}
	progress := state.Progress
	text := fmt.Sprintf("⏳ %s: готово %d из %d", importName(&state.Batch), progress.Done+progress.Failed, state.Batch.Total)
	if progress.Failed > 0 {
		text += fmt.Sprintf(", ошибок %d", progress.Failed)
	}
	return text
}

func batchSummary(state *ingest_track.BatchState) string {
	var text strings.Builder
	fmt.Fprintf(&text, "✅ %s: %d из %s в библиотеке", importName(&state.Batch), state.Progress.Done, plural(state.Batch.Total, "трека", "треков", "треков"))
	failed := state.FailedNames
	if len(failed) > 0 {
		fmt.Fprintf(&text, "\n\n❌ Не удалось загрузить (%d):", len(failed))
		for _, name := range failed[:min(len(failed), maxFailedListed)] {
			fmt.Fprintf(&text, "\n• %s", html.EscapeString(name))
		}
		if len(failed) > maxFailedListed {
			fmt.Fprintf(&text, "\n…и ещё %d", len(failed)-maxFailedListed)
		}
	}
	return text.String()
}
