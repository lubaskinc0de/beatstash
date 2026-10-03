package view_top

import (
	"context"
	"time"

	"github.com/lubaskinc0de/beatstash/internal/application/common"
	"github.com/lubaskinc0de/beatstash/internal/application/common/repositories"
)

const topLimit = 10

type Top struct {
	Sharers      Rating
	TakenAuthors Rating
}

type Rating struct {
	AllTime   []repositories.TopEntry
	ThisMonth []repositories.TopEntry
}

type GetTop struct {
	IDs    common.IDProvider
	Shared repositories.SharedTracks
	Takes  repositories.Takes
	Clock  func() time.Time
}

func (i *GetTop) Execute(ctx context.Context) (*Top, error) {
	if _, err := i.IDs.CurrentUser(ctx); err != nil {
		return nil, err
	}

	now := i.Clock()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var top Top
	for _, query := range []struct {
		fetch func(context.Context, time.Time, int) ([]repositories.TopEntry, error)
		since time.Time
		into  *[]repositories.TopEntry
	}{
		{i.Shared.TopSharers, time.Time{}, &top.Sharers.AllTime},
		{i.Shared.TopSharers, monthStart, &top.Sharers.ThisMonth},
		{i.Takes.TopTaken, time.Time{}, &top.TakenAuthors.AllTime},
		{i.Takes.TopTaken, monthStart, &top.TakenAuthors.ThisMonth},
	} {
		entries, err := query.fetch(ctx, query.since, topLimit)
		if err != nil {
			return nil, err
		}
		*query.into = entries
	}
	return &top, nil
}
