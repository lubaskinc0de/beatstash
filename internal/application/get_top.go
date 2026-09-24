package application

import (
	"context"
	"time"
)

const topLimit = 10

type Top struct {
	Sharers      Rating
	TakenAuthors Rating
}

type Rating struct {
	AllTime   []TopEntry
	ThisMonth []TopEntry
}

type GetTop struct {
	Shares ShareRepository
	Takes  TakeRepository
	Clock  func() time.Time
}

func NewGetTop(shares ShareRepository, takes TakeRepository, clock func() time.Time) *GetTop {
	return &GetTop{Shares: shares, Takes: takes, Clock: clock}
}

func (i *GetTop) Execute(ctx context.Context) (*Top, error) {
	now := i.Clock()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var top Top
	for _, query := range []struct {
		fetch func(context.Context, time.Time, int) ([]TopEntry, error)
		since time.Time
		into  *[]TopEntry
	}{
		{i.Shares.TopSharers, time.Time{}, &top.Sharers.AllTime},
		{i.Shares.TopSharers, monthStart, &top.Sharers.ThisMonth},
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
