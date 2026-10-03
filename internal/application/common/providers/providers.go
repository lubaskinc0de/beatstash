package providers

import (
	"context"
	"io"

	"github.com/lubaskinc0de/beatstash/internal/domain/library"
	"github.com/lubaskinc0de/beatstash/internal/domain/provider"
)

// Provider is an external source of music. What it can do is expressed by
// the Capability interfaces it implements.
type Provider interface {
	Name() provider.ProviderName
}

// Fetcher and the other Capabilities get the user they act for: the
// Provider may need their Provider Account.
type Fetcher interface {
	Fetch(ctx context.Context, userID uint, ref provider.TrackRef) (*FetchedAudio, error)
}

// Describer tells what a track is before it is fetched, so a track the user
// has already need not be downloaded.
type Describer interface {
	// Describe returns nil if the Provider knows no such track.
	Describe(ctx context.Context, userID uint, ref provider.TrackRef) (*Description, error)
}

type Description struct {
	Metadata   library.Metadata
	DurationMs int
}

// Releaser frees what Fetch left on the Provider side. It is called once
// the job is finished for good, so retries can fetch again.
type Releaser interface {
	Release(ctx context.Context, ref provider.TrackRef) error
}

type TokenChecker interface {
	CheckToken(ctx context.Context, token string) error
}

type CollectionLister interface {
	Collection(ctx context.Context, userID uint) (*Collection, error)
}

type FetchedAudio struct {
	// Body is the audio stream; the caller closes it.
	Body     io.ReadCloser
	FileName string
	Format   library.Format

	// Hint is what the Provider knows for sure, it beats the file's tags.
	Hint library.Metadata
	// WeakHint is used only for fields the file's tags lack.
	WeakHint library.Metadata
	Cover    []byte
}

// Recognizer tells the Tracks whose file the Track Ref is, if the service
// itself gave that file out: then no download is needed.
type Recognizer interface {
	Recognize(ctx context.Context, ref provider.TrackRef) ([]uint, error)
}
