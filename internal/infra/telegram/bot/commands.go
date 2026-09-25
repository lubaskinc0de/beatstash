package bot

import (
	"strings"

	"github.com/go-telegram/bot/models"
)

// command splits "/name@bot args" into name and args.
func command(update *models.Update) (name, args string, ok bool) {
	if update.Message == nil || !strings.HasPrefix(update.Message.Text, "/") {
		return "", "", false
	}

	head, args, _ := strings.Cut(update.Message.Text, " ")
	name, _, _ = strings.Cut(strings.TrimPrefix(head, "/"), "@")
	return strings.ToLower(name), strings.TrimSpace(args), true
}

func isCommand(name string) func(*models.Update) bool {
	return func(update *models.Update) bool {
		got, _, ok := command(update)
		return ok && got == name
	}
}
