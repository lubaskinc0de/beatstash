package navidrome

import "errors"

var (
	ErrInvalidCredentials = errors.New("navidrome: invalid credentials")
	ErrLoginTaken         = errors.New("navidrome: login taken")
	ErrNameTaken          = errors.New("navidrome: library name taken")
	ErrAccountNotFound    = errors.New("navidrome: no such account")
)

// ErrAdminAccount: Navidrome shows admins every library, so their access cannot be narrowed.
var ErrAdminAccount = errors.New("navidrome: account is an admin")
