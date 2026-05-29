package interactions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/richaardev/concord"
	"github.com/richaardev/concord/interactivity"
	"github.com/richaardev/discord-backuper/internal/backup"
	"github.com/richaardev/discord-backuper/internal/database/sqlc"
)

func SlashBackup(q *sqlc.Queries, db *sql.DB) *concord.SlashCommand {
	return &concord.SlashCommand{
		Name:        "backup",
		Description: "Manage Discord server backups",
		SubCommands: []concord.SubCommand{
			{
				Name:        "create",
				Description: "Create a backup of the server",
				RunE:        handleCreate(q),
			},
			{
				Name:        "info",
				Description: "Show backup information",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "id",
						Description: "The backup identifier",
						Required:    true,
					},
				},
				RunE: handleInfo(q),
			},
			{
				Name:        "list",
				Description: "List all available backups",
				RunE:        handleList(q),
			},
			{
				Name:        "apply",
				Description: "Apply a backup to the server",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "id",
						Description: "The backup identifier",
						Required:    true,
					},
				},
				RunE: handleApply(q, db),
			},
			{
				Name:        "delete",
				Description: "Delete a backup",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "id",
						Description: "The backup identifier",
						Required:    true,
					},
				},
				RunE: handleDelete(q),
			},
		},
	}
}

func guildIDFromEvent(event *events.ApplicationCommandInteractionCreate) snowflake.ID {
	if event.GuildID() == nil {
		return 0
	}
	return *event.GuildID()
}

func handleCreate(q *sqlc.Queries) func(event *events.ApplicationCommandInteractionCreate) error {
	return func(event *events.ApplicationCommandInteractionCreate) error {
		guildID := guildIDFromEvent(event)

		if guildID == 0 {
			return event.CreateMessage(discord.MessageCreate{
				Content: "This command can only be used in a server.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		backupID := fmt.Sprintf("backup_%s", time.Now().Format("20060102_150405"))
		view := backup.NewCreatePanel(backupID, guildID, event.Client().Rest, q)
		panel := interactivity.NewPanel(
			event.Client(), view,
			interactivity.WithOptions(interactivity.PanelOptions{
				IdleTime: 5 * time.Minute,
			}),
		)
		return panel.Render(event)
	}
}

func handleInfo(q *sqlc.Queries) func(event *events.ApplicationCommandInteractionCreate) error {
	return func(event *events.ApplicationCommandInteractionCreate) error {
		data := event.SlashCommandInteractionData()
		backupID, _ := data.OptString("id")

		b, err := q.GetBackup(context.Background(), backupID)
		if err != nil {
			if err == sql.ErrNoRows {
				return event.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Backup `%s` not found.", backupID),
					Flags:   discord.MessageFlagEphemeral,
				})
			}
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error fetching backup: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		var d backup.BackupData
		if err := json.Unmarshal([]byte(b.Data), &d); err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error decoding backup data: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		embed := discord.Embed{
			Title: fmt.Sprintf("📦 Backup: %s", backupID),
			Color: 0x5865F2,
			Fields: []discord.EmbedField{
				{Name: "Guild ID", Value: b.GuildID, Inline: new(true)},
				{Name: "Created", Value: b.CreatedAt, Inline: new(true)},
				{Name: "Updated", Value: b.UpdatedAt, Inline: new(true)},
				{Name: "Roles", Value: fmt.Sprintf("%d", len(d.Roles)), Inline: new(true)},
				{Name: "Channels", Value: fmt.Sprintf("%d", len(d.Channels)), Inline: new(true)},
				{Name: "Bans", Value: fmt.Sprintf("%d", len(d.Bans)), Inline: new(true)},
			},
		}

		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed},
			Flags:  discord.MessageFlagEphemeral,
		})
	}
}

func handleList(q *sqlc.Queries) func(event *events.ApplicationCommandInteractionCreate) error {
	return func(event *events.ApplicationCommandInteractionCreate) error {
		guildID := guildIDFromEvent(event)
		if guildID == 0 {
			return event.CreateMessage(discord.MessageCreate{
				Content: "This command can only be used in a server.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		backups, err := q.ListBackups(context.Background(), guildID.String())
		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error listing backups: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		if len(backups) == 0 {
			return event.CreateMessage(discord.MessageCreate{
				Content: "📭 No backups found for this server.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		var content strings.Builder
		fmt.Fprintf(&content, "**📋 Backups for this server (%d total)**\n\n", len(backups))
		for _, b := range backups {
			var d backup.BackupData
			if err := json.Unmarshal([]byte(b.Data), &d); err != nil {
				continue
			}
			fmt.Fprintf(&content, "• `%s` — %d roles, %d channels, %d bans *(created: %s)*\n",
				b.ID, len(d.Roles), len(d.Channels), len(d.Bans), b.CreatedAt)
		}

		return event.CreateMessage(discord.MessageCreate{
			Content: content.String(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}
}

func handleApply(q *sqlc.Queries, db *sql.DB) func(event *events.ApplicationCommandInteractionCreate) error {
	return func(event *events.ApplicationCommandInteractionCreate) error {
		data := event.SlashCommandInteractionData()
		backupID, _ := data.OptString("id")
		guildID := guildIDFromEvent(event)

		if guildID == 0 {
			return event.CreateMessage(discord.MessageCreate{
				Content: "This command can only be used in a server.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		b, err := q.GetBackup(context.Background(), backupID)
		if err != nil {
			if err == sql.ErrNoRows {
				return event.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Backup `%s` not found.", backupID),
					Flags:   discord.MessageFlagEphemeral,
				})
			}
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error fetching backup: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		var d backup.BackupData
		if err := json.Unmarshal([]byte(b.Data), &d); err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error decoding backup data: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		view := backup.NewApplyPanel(backupID, &d, guildID, event.Client().Rest)
		panel := interactivity.NewPanel(
			event.Client(), view,
			interactivity.WithOptions(interactivity.PanelOptions{
				IdleTime: 10 * time.Minute,
			}),
		)
		return panel.Render(event)
	}
}

func handleDelete(q *sqlc.Queries) func(event *events.ApplicationCommandInteractionCreate) error {
	return func(event *events.ApplicationCommandInteractionCreate) error {
		data := event.SlashCommandInteractionData()
		backupID, _ := data.OptString("id")

		_, err := q.GetBackup(context.Background(), backupID)
		if err != nil {
			if err == sql.ErrNoRows {
				return event.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Backup `%s` not found.", backupID),
					Flags:   discord.MessageFlagEphemeral,
				})
			}
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error fetching backup: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		if err := q.DeleteBackup(context.Background(), backupID); err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error deleting backup: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("✅ Backup `%s` has been deleted.", backupID),
			Flags:   discord.MessageFlagEphemeral,
		})
	}
}
