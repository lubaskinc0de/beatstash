// The errors of Provider Accounts.

package provider

import "errors"

var (
	ErrAccountDisconnected = errors.New("provider account is disconnected")
	ErrTokenRejected       = errors.New("provider rejects the token")
)
