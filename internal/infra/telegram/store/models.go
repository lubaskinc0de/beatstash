// Package store keeps what the Telegram adapter remembers of its chats,
// in tables of its own: the core knows nothing of chats and messages.
package store

var Models = []any{
	&Dialog{},
	&JobMessage{},
	&BatchMessage{},
	&AccountNotice{},
	&File{},
}
