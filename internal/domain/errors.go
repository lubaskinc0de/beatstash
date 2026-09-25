package domain

import "errors"

var (
	ErrNotOwnTrack      = errors.New("track is not in the user's Personal Library")
	ErrInboxTrack       = errors.New("inbox track cannot be shared")
	ErrNotShared        = errors.New("track is not in the Shared Library")
	ErrAlreadyInLibrary = errors.New("the track is in the user's Personal Library already")
)
