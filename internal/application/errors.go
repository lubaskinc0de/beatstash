package application

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrTrackNotFound = errors.New("track not found")
var ErrNotAuthenticated = errors.New("access denied")
