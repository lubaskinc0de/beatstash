// The errors the rules of Libraries and Tracks return.

package library

import "errors"

var (
	ErrNotKeptTrack     = errors.New("track is in none of the user's libraries")
	ErrInboxTrack       = errors.New("inbox track cannot be shared")
	ErrAlreadyInLibrary = errors.New("the user has the track already")
	ErrNotPersonal      = errors.New("only a Personal Library has a Quota of its own")
)
