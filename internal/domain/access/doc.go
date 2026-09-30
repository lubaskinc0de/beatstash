// Package access: users and how they get access to the bot.
//
// Aggregates:
//   - User. Its Identities are part of it. Admin comes from the config.
//   - Invite: works once and only until it expires. Only an Admin creates one.
//   - NavidromeAccount: the login the bot uses in Navidrome for the User.
//
// Value objects: Identity, Channel, Profile. Last Seen is kept on the User
// and saved at most once a minute.
//
// Fields are exported only because gorm needs them. Change an aggregate
// only through its methods.
package access
