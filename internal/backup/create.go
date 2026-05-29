package backup

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

func Create(guildID snowflake.ID, restClient rest.Rest) (*BackupData, error) {
	slog.Info("fetching guild channels...")
	channels, err := restClient.GetGuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channels: %w", err)
	}
	slog.Info("fetched channels", "count", len(channels))

	slog.Info("fetching guild roles...")
	roles, err := restClient.GetRoles(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}
	slog.Info("fetched roles", "count", len(roles))

	slog.Info("fetching guild bans...")
	bans, err := restClient.GetBans(guildID, 0, 0, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get bans: %w", err)
	}
	slog.Info("fetched bans", "count", len(bans))

	data := BackupData{
		Version:  1,
		GuildID:  guildID.String(),
		Roles:    make([]RoleBackup, 0, len(roles)),
		Channels: make([]ChannelBackup, 0, len(channels)),
		Bans:     make([]BanBackup, 0, len(bans)),
	}

	for _, role := range roles {
		if role.ID == guildID {
			continue
		}

		r := RoleBackup{
			ID:          role.ID.String(),
			Name:        role.Name,
			Color:       role.Color,
			Hoist:       role.Hoist,
			Position:    role.Position,
			Permissions: int64(role.Permissions),
			Mentionable: role.Mentionable,
			Managed:     role.Managed,
		}
		if role.Icon != nil {
			r.Icon = *role.Icon
		}
		if role.Emoji != nil {
			r.Emoji = *role.Emoji
		}
		data.Roles = append(data.Roles, r)
	}

	for _, ch := range channels {
	cb := ChannelBackup{
			ID:       ch.ID().String(),
			Type:     int(ch.Type()),
			Name:     ch.Name(),
			Position: ch.Position(),
		}

		if parentID := ch.ParentID(); parentID != nil {
			cb.ParentID = parentID.String()
		}

		switch v := ch.(type) {
		case discord.GuildTextChannel:
			if topic := v.Topic(); topic != nil {
				cb.Topic = *topic
			}
			cb.NSFW = v.NSFW()
			cb.RateLimitPerUser = v.RateLimitPerUser()
		case discord.GuildVoiceChannel:
			cb.Bitrate = v.Bitrate()
			cb.UserLimit = v.UserLimit
		case discord.GuildNewsChannel:
			if topic := v.Topic(); topic != nil {
				cb.Topic = *topic
			}
			cb.NSFW = v.NSFW()
			cb.RateLimitPerUser = v.RateLimitPerUser()
		case discord.GuildStageVoiceChannel:
			cb.Bitrate = v.Bitrate()
			cb.UserLimit = 0
		case discord.GuildForumChannel:
			if v.Topic != nil {
				cb.Topic = *v.Topic
			}
			cb.NSFW = v.NSFW
			cb.RateLimitPerUser = v.RateLimitPerUser
		case discord.GuildMediaChannel:
			if v.Topic != nil {
				cb.Topic = *v.Topic
			}
			cb.NSFW = v.NSFW
		}

		overwrites := ch.PermissionOverwrites()
		if len(overwrites) > 0 {
			cb.PermissionOverwrites = make([]PermissionOverwriteBackup, 0, len(overwrites))
			for _, ow := range overwrites {
				pob := PermissionOverwriteBackup{
					ID:   ow.ID().String(),
					Type: int(ow.Type()),
				}
				switch v := ow.(type) {
				case discord.RolePermissionOverwrite:
					pob.Allow = int64(v.Allow)
					pob.Deny = int64(v.Deny)
				case discord.MemberPermissionOverwrite:
					pob.Allow = int64(v.Allow)
					pob.Deny = int64(v.Deny)
				}
				cb.PermissionOverwrites = append(cb.PermissionOverwrites, pob)
			}
		}

		data.Channels = append(data.Channels, cb)
	}

	for _, ban := range bans {
		bb := BanBackup{
			UserID:   ban.User.ID.String(),
			Username: ban.User.Username,
		}
		if ban.Reason != nil {
			bb.Reason = *ban.Reason
		}
		data.Bans = append(data.Bans, bb)
	}

	raw, _ := json.Marshal(data)
	slog.Info("backup data size", "bytes", len(raw))

	return &data, nil
}

func (d *BackupData) Summary() string {
	return fmt.Sprintf(
		"📦 **Backup Summary**\n• Roles: %d\n• Channels: %d\n• Bans: %d\n• Created: %s",
		len(d.Roles), len(d.Channels), len(d.Bans), time.Now().Format("2006-01-02 15:04:05"),
	)
}
