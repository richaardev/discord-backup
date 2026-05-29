package bot

import (
	"fmt"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/richaardev/concord"
	"github.com/richaardev/concord/cordutils"
)

func OnReady(client *BotClient) bot.EventListener {
	return bot.NewListenerFunc(func(event *events.Ready) {
		create := []discord.ApplicationCommandCreate{}

		for _, c := range client.InteractionManager.ListSlashCommands() {
			slashCreate := concord.SlashCommandToCreate(c)
			create = append(create, slashCreate)
		}

		commands, err := event.Client().Rest.GetGlobalCommands(event.Client().ID(), false)
		if err != nil {
			return
		}

		equal := cordutils.CheckIfCommandsIsEqual(commands, create)
		if !equal {
			slog.Info("updating application commands", slog.Int("total", len(create)))
			_, err := event.Client().Rest.SetGlobalCommands(event.Client().ID(), create)
			if err != nil {
				slog.Error("error while setting global commands", slog.Any("err", err))
			}
		}

		slog.Info(fmt.Sprintf("%s is now online!", event.User.Username))
	})
}
