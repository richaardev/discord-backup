package backup

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"
)

type ApplyResult struct {
	RolesCreated    int
	ChannelsCreated int
	BansApplied     int
	Errors          []string
}

type ApplyConfig struct {
	RestoreRoles    bool
	RestoreChannels bool
	RestoreBans     bool
	ProtectedRoles  []snowflake.ID
	ProtectedChannels []snowflake.ID
}

func Apply(guildID snowflake.ID, data *BackupData, cfg ApplyConfig, restClient rest.Rest) *ApplyResult {
	result := &ApplyResult{}

	oldToNewRoleID := make(map[string]snowflake.ID)

	existingRoles, _ := restClient.GetRoles(guildID)
	existingChannels, _ := restClient.GetGuildChannels(guildID)

	protectedRoleIDs := make(map[snowflake.ID]bool)
	for _, id := range cfg.ProtectedRoles {
		protectedRoleIDs[id] = true
	}
	protectedChannelIDs := make(map[snowflake.ID]bool)
	for _, id := range cfg.ProtectedChannels {
		protectedChannelIDs[id] = true
	}

	existingRoleByName := make(map[string]discord.Role)
	for _, role := range existingRoles {
		existingRoleByName[role.Name] = role
	}

	if cfg.RestoreRoles {
		skipRoleNames := make(map[string]bool)
		for _, rb := range data.Roles {
			if existingRole, ok := existingRoleByName[rb.Name]; ok {
				if protectedRoleIDs[existingRole.ID] {
					skipRoleNames[rb.Name] = true
					oldToNewRoleID[rb.ID] = existingRole.ID
				}
			}
		}

		for _, role := range existingRoles {
			if role.ID == guildID {
				continue
			}
			if role.Managed {
				continue
			}
			if protectedRoleIDs[role.ID] {
				continue
			}

			slog.Info("deleting existing role", "name", role.Name)
			if err := restClient.DeleteRole(guildID, role.ID); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete role %q: %v", role.Name, err))
			}
			time.Sleep(200 * time.Millisecond)
		}

		for _, rb := range data.Roles {
			if skipRoleNames[rb.Name] {
				slog.Info("skipping protected role", "name", rb.Name)
				continue
			}
			if rb.Managed {
				slog.Info("skipping managed role", "name", rb.Name)
				continue
			}

			create := discord.RoleCreate{
				Name:        rb.Name,
				Color:       rb.Color,
				Hoist:       rb.Hoist,
				Mentionable: rb.Mentionable,
			}
			perms := discord.Permissions(rb.Permissions)
			create.Permissions = &perms

			slog.Info("creating role", "name", rb.Name)
			role, err := restClient.CreateRole(guildID, create)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create role %q: %v", rb.Name, err))
				continue
			}
			oldToNewRoleID[rb.ID] = role.ID
			result.RolesCreated++
			time.Sleep(200 * time.Millisecond)
		}

		rolePositions := make([]discord.RolePositionUpdate, 0)
		for _, rb := range data.Roles {
			if skipRoleNames[rb.Name] || rb.Managed {
				continue
			}
			if newID, ok := oldToNewRoleID[rb.ID]; ok {
				pos := rb.Position
				rolePositions = append(rolePositions, discord.RolePositionUpdate{
					ID:       newID,
					Position: &pos,
				})
			}
		}
		if len(rolePositions) > 0 {
			slog.Info("updating role positions")
			if _, err := restClient.UpdateRolePositions(guildID, rolePositions); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to update role positions: %v", err))
			}
		}
	}

	if cfg.RestoreChannels {
		for _, ch := range existingChannels {
			if protectedChannelIDs[ch.ID()] {
				continue
			}
			slog.Info("deleting existing channel", "name", ch.Name())
			if err := restClient.DeleteChannel(ch.ID()); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to delete channel %q: %v", ch.Name(), err))
			}
			time.Sleep(200 * time.Millisecond)
		}

		categoriesFirst := make([]ChannelBackup, 0)
		channelsAfter := make([]ChannelBackup, 0)
		for _, cb := range data.Channels {
			switch discord.ChannelType(cb.Type) {
			case discord.ChannelTypeGuildCategory:
				categoriesFirst = append(categoriesFirst, cb)
			default:
				channelsAfter = append(channelsAfter, cb)
			}
		}

		oldToNewChannelID := make(map[string]snowflake.ID)

		for _, cb := range append(categoriesFirst, channelsAfter...) {
			if protectedChannelIDs[snowflake.MustParse(cb.ID)] {
				slog.Info("skipping protected channel", "name", cb.Name)
				continue
			}

			var parentID snowflake.ID
			if cb.ParentID != "" {
				if pid, ok := oldToNewChannelID[cb.ParentID]; ok {
					parentID = pid
				}
			}
			create := channelBackupToCreate(cb, oldToNewRoleID, parentID)

			slog.Info("creating channel", "name", cb.Name, "type", discord.ChannelType(cb.Type))
			ch, err := restClient.CreateGuildChannel(guildID, create)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create channel %q: %v", cb.Name, err))
				continue
			}
			oldToNewChannelID[cb.ID] = ch.ID()
			result.ChannelsCreated++
			time.Sleep(200 * time.Millisecond)
		}

		channelPositions := make([]discord.GuildChannelPositionUpdate, 0)
		for _, cb := range data.Channels {
			if newID, ok := oldToNewChannelID[cb.ID]; ok {
				channelPositions = append(channelPositions, discord.GuildChannelPositionUpdate{
					ID:       newID,
					Position: omit.NewPtr(cb.Position),
				})
			}
		}
		if len(channelPositions) > 0 {
			slog.Info("updating channel positions")
			if err := restClient.UpdateChannelPositions(guildID, channelPositions); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to update channel positions: %v", err))
			}
		}
	}

	if cfg.RestoreBans {
		for _, bb := range data.Bans {
			userID := snowflake.MustParse(bb.UserID)
			slog.Info("applying ban", "username", bb.Username)
			if err := restClient.AddBan(guildID, userID, 0); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to ban %q: %v", bb.Username, err))
				continue
			}
			result.BansApplied++
			time.Sleep(200 * time.Millisecond)
		}
	}

	return result
}

func channelBackupToCreate(cb ChannelBackup, roleIDMap map[string]snowflake.ID, parentID snowflake.ID) discord.GuildChannelCreate {
	overwrites := make([]discord.PermissionOverwrite, 0)
	for _, ow := range cb.PermissionOverwrites {
		var newID snowflake.ID
		if discord.PermissionOverwriteType(ow.Type) == discord.PermissionOverwriteTypeRole {
			if mapped, ok := roleIDMap[ow.ID]; ok {
				newID = mapped
			} else {
				newID = snowflake.MustParse(ow.ID)
			}
		} else {
			newID = snowflake.MustParse(ow.ID)
		}

		switch discord.PermissionOverwriteType(ow.Type) {
		case discord.PermissionOverwriteTypeRole:
			overwrites = append(overwrites, discord.RolePermissionOverwrite{
				RoleID: newID,
				Allow:  discord.Permissions(ow.Allow),
				Deny:   discord.Permissions(ow.Deny),
			})
		case discord.PermissionOverwriteTypeMember:
			overwrites = append(overwrites, discord.MemberPermissionOverwrite{
				UserID: newID,
				Allow:  discord.Permissions(ow.Allow),
				Deny:   discord.Permissions(ow.Deny),
			})
		}
	}

	switch discord.ChannelType(cb.Type) {
	case discord.ChannelTypeGuildText:
		return discord.GuildTextChannelCreate{
			Name:                 cb.Name,
			Topic:                cb.Topic,
			NSFW:                 cb.NSFW,
			RateLimitPerUser:     cb.RateLimitPerUser,
			PermissionOverwrites: overwrites,
			ParentID:             parentID,
		}
	case discord.ChannelTypeGuildVoice:
		return discord.GuildVoiceChannelCreate{
			Name:                 cb.Name,
			Bitrate:              cb.Bitrate,
			UserLimit:            cb.UserLimit,
			PermissionOverwrites: overwrites,
			ParentID:             parentID,
		}
	case discord.ChannelTypeGuildCategory:
		return discord.GuildCategoryChannelCreate{
			Name:                 cb.Name,
			PermissionOverwrites: overwrites,
		}
	case discord.ChannelTypeGuildNews:
		return discord.GuildNewsChannelCreate{
			Name:                 cb.Name,
			Topic:                cb.Topic,
			NSFW:                 cb.NSFW,
			RateLimitPerUser:     cb.RateLimitPerUser,
			PermissionOverwrites: overwrites,
			ParentID:             parentID,
		}
	case discord.ChannelTypeGuildStageVoice:
		return discord.GuildStageVoiceChannelCreate{
			Name:                 cb.Name,
			Bitrate:              cb.Bitrate,
			PermissionOverwrites: overwrites,
			ParentID:             parentID,
		}
	case discord.ChannelTypeGuildForum:
		return discord.GuildForumChannelCreate{
			Name:                 cb.Name,
			Topic:                cb.Topic,
			RateLimitPerUser:     cb.RateLimitPerUser,
			PermissionOverwrites: overwrites,
			ParentID:             parentID,
		}
	case discord.ChannelTypeGuildMedia:
		return discord.GuildMediaChannelCreate{
			Name:                 cb.Name,
			PermissionOverwrites: overwrites,
			ParentID:             parentID,
		}
	default:
		return discord.GuildTextChannelCreate{
			Name:     cb.Name,
			ParentID: parentID,
		}
	}
}
