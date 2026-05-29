package backup

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/richaardev/concord/interactivity"
)

type applyState int

const (
	stateConfigMain applyState = iota
	stateSelectRoles
	stateSelectChannels
	stateConfirm
	stateApplying
	stateDone
)

type ApplyPanel struct {
	state applyState

	backupID string
	data     *BackupData
	guildID  snowflake.ID
	rest     rest.Rest

	restoreRoles    bool
	restoreChannels bool
	restoreBans     bool

	protectedRoles    map[snowflake.ID]bool
	protectedChannels map[snowflake.ID]bool

	allRoles    []discord.Role
	allChannels []discord.GuildChannel

	rolePage    int
	channelPage int

	result *ApplyResult
}

func NewApplyPanel(backupID string, data *BackupData, guildID snowflake.ID, rest rest.Rest) *ApplyPanel {
	return &ApplyPanel{
		state:             stateConfigMain,
		backupID:          backupID,
		data:              data,
		guildID:           guildID,
		rest:              rest,
		restoreRoles:      true,
		restoreChannels:   true,
		restoreBans:       true,
		protectedRoles:    make(map[snowflake.ID]bool),
		protectedChannels: make(map[snowflake.ID]bool),
	}
}

func (p *ApplyPanel) Init() {
	roles, _ := p.rest.GetRoles(p.guildID)
	p.allRoles = roles

	channels, _ := p.rest.GetGuildChannels(p.guildID)
	p.allChannels = channels
}

func (p *ApplyPanel) Update(e any) (interactivity.PanelModel, interactivity.Cmd) {
	switch evt := e.(type) {
	case *events.ComponentInteractionCreate:
		return p.handleComponent(evt)
	}
	return p, nil
}

func (p *ApplyPanel) handleComponent(e *events.ComponentInteractionCreate) (interactivity.PanelModel, interactivity.Cmd) {
	customID := interactivity.GetCustomID(e.Data.CustomID())

	switch p.state {
	case stateConfigMain:
		return p.handleConfigMain(customID, e)
	case stateSelectRoles:
		return p.handleSelectRoles(customID, e)
	case stateSelectChannels:
		return p.handleSelectChannels(customID, e)
	case stateConfirm:
		return p.handleConfirm(customID, e)
	case stateApplying:
		return nil, nil
	case stateDone:
		return nil, nil
	}
	return p, nil
}

func (p *ApplyPanel) handleConfigMain(customID string, e *events.ComponentInteractionCreate) (interactivity.PanelModel, interactivity.Cmd) {
	switch customID {
	case "toggle:roles":
		p.restoreRoles = !p.restoreRoles
	case "toggle:channels":
		p.restoreChannels = !p.restoreChannels
	case "toggle:bans":
		p.restoreBans = !p.restoreBans
	case "select:roles":
		p.state = stateSelectRoles
		p.rolePage = 0
		return p, nil
	case "select:channels":
		p.state = stateSelectChannels
		p.channelPage = 0
		return p, nil
	case "apply:start":
		p.state = stateConfirm
	case "cancel":
		return nil, nil
	}
	return p, nil
}

func (p *ApplyPanel) handleSelectRoles(customID string, e *events.ComponentInteractionCreate) (interactivity.PanelModel, interactivity.Cmd) {
	switch customID {
	case "roles:back":
		p.state = stateConfigMain
	case "roles:prev":
		if p.rolePage > 0 {
			p.rolePage--
		}
	case "roles:next":
		itemsPerPage := 25
		totalRoles := len(p.allRoles)
		if (p.rolePage+1)*itemsPerPage < totalRoles {
			p.rolePage++
		}
	}

	if selectData, ok := e.Data.(discord.RoleSelectMenuInteractionData); ok {
		p.protectedRoles = make(map[snowflake.ID]bool)
		for _, id := range selectData.Values {
			p.protectedRoles[id] = true
		}
	}

	return p, nil
}

func (p *ApplyPanel) handleSelectChannels(customID string, e *events.ComponentInteractionCreate) (interactivity.PanelModel, interactivity.Cmd) {
	switch customID {
	case "channels:back":
		p.state = stateConfigMain
	case "channels:prev":
		if p.channelPage > 0 {
			p.channelPage--
		}
	case "channels:next":
		itemsPerPage := 25
		totalChannels := len(p.allChannels)
		if (p.channelPage+1)*itemsPerPage < totalChannels {
			p.channelPage++
		}
	}

	if selectData, ok := e.Data.(discord.ChannelSelectMenuInteractionData); ok {
		p.protectedChannels = make(map[snowflake.ID]bool)
		for _, id := range selectData.Values {
			p.protectedChannels[id] = true
		}
	}

	return p, nil
}

func (p *ApplyPanel) handleConfirm(customID string, e *events.ComponentInteractionCreate) (interactivity.PanelModel, interactivity.Cmd) {
	switch customID {
	case "confirm:yes":
		go p.runApply()
		return p, nil
	case "confirm:no":
		p.state = stateConfigMain
		return p, nil
	}
	return p, nil
}

func (p *ApplyPanel) runApply() {
	p.state = stateApplying

	protectedRoleSlice := make([]snowflake.ID, 0, len(p.protectedRoles))
	for id := range p.protectedRoles {
		protectedRoleSlice = append(protectedRoleSlice, id)
	}

	protectedChannelSlice := make([]snowflake.ID, 0, len(p.protectedChannels))
	for id := range p.protectedChannels {
		protectedChannelSlice = append(protectedChannelSlice, id)
	}

	cfg := ApplyConfig{
		RestoreRoles:      p.restoreRoles,
		RestoreChannels:   p.restoreChannels,
		RestoreBans:       p.restoreBans,
		ProtectedRoles:    protectedRoleSlice,
		ProtectedChannels: protectedChannelSlice,
	}

	p.result = Apply(p.guildID, p.data, cfg, p.rest)
	p.state = stateDone

	slog.Info("backup apply completed", "backup_id", p.backupID, "roles", p.result.RolesCreated, "channels", p.result.ChannelsCreated, "bans", p.result.BansApplied)

	time.Sleep(5 * time.Second)
}

func (p *ApplyPanel) View() interactivity.Message {
	switch p.state {
	case stateConfigMain:
		return p.viewConfigMain()
	case stateSelectRoles:
		return p.viewSelectRoles()
	case stateSelectChannels:
		return p.viewSelectChannels()
	case stateConfirm:
		return p.viewConfirm()
	case stateApplying:
		return p.viewApplying()
	case stateDone:
		return p.viewDone()
	}
	return interactivity.Message{}
}

func (p *ApplyPanel) viewConfigMain() interactivity.Message {
	toggleRoles := discord.NewSuccessButton("✅ Roles", "toggle:roles")
	if !p.restoreRoles {
		toggleRoles = discord.NewSecondaryButton("⬜ Roles", "toggle:roles")
	}

	toggleChannels := discord.NewSuccessButton("✅ Channels", "toggle:channels")
	if !p.restoreChannels {
		toggleChannels = discord.NewSecondaryButton("⬜ Channels", "toggle:channels")
	}

	toggleBans := discord.NewSuccessButton("✅ Bans", "toggle:bans")
	if !p.restoreBans {
		toggleBans = discord.NewSecondaryButton("⬜ Bans", "toggle:bans")
	}

	components := []discord.LayoutComponent{
		discord.NewContainer(
			discord.NewTextDisplay("### ⚙️ Backup Apply Configuration"),
			discord.NewTextDisplay(fmt.Sprintf("Configure how to apply backup **%s**", p.backupID)),
			discord.NewTextDisplay(fmt.Sprintf("**Roles:** %d roles in backup", len(p.data.Roles))),
			discord.NewTextDisplay(fmt.Sprintf("**Channels:** %d channels in backup", len(p.data.Channels))),
			discord.NewTextDisplay(fmt.Sprintf("**Bans:** %d bans in backup", len(p.data.Bans))),
		).WithAccentColor(0x5865F2),
		discord.NewTextDisplay("### Toggle categories to restore"),
		discord.NewActionRow(toggleRoles, toggleChannels, toggleBans),
	}

	if p.restoreRoles {
		components = append(components,
			discord.NewTextDisplay(fmt.Sprintf("**Protected roles:** %d selected", len(p.protectedRoles))),
			discord.NewActionRow(
				discord.NewSecondaryButton("⚙️ Select roles to protect", "select:roles"),
			),
		)
	}

	if p.restoreChannels {
		components = append(components,
			discord.NewTextDisplay(fmt.Sprintf("**Protected channels:** %d selected", len(p.protectedChannels))),
			discord.NewActionRow(
				discord.NewSecondaryButton("⚙️ Select channels to protect", "select:channels"),
			),
		)
	}

	components = append(components,
		discord.NewActionRow(
			discord.NewSuccessButton("▶️ Apply Backup", "apply:start"),
			discord.NewDangerButton("❌ Cancel", "cancel"),
		),
	)

	return interactivity.Message{
		Components: &components,
		Flags:      new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *ApplyPanel) viewSelectRoles() interactivity.Message {
	itemsPerPage := 25
	start := p.rolePage * itemsPerPage
	end := start + itemsPerPage
	if end > len(p.allRoles) {
		end = len(p.allRoles)
	}

	pageRoles := p.allRoles[start:end]
	totalPages := (len(p.allRoles) + itemsPerPage - 1) / itemsPerPage

	selectMenu := discord.NewRoleSelectMenu("roles:select", "Choose roles to protect...").
		WithMaxValues(25)

	components := []discord.LayoutComponent{
		discord.NewContainer(discord.NewTextDisplay("### 🛡️ Select Roles to Protect"), discord.NewTextDisplay(fmt.Sprintf("Page %d/%d — Select roles that should NOT be deleted.", p.rolePage+1, totalPages))).WithAccentColor(0x5865F2),
		discord.NewTextDisplay(fmt.Sprintf("### Page %d/%d — %d roles shown", p.rolePage+1, totalPages, len(pageRoles))),
		discord.NewActionRow(selectMenu),
	}

	var navButtons []discord.InteractiveComponent
	if p.rolePage > 0 {
		navButtons = append(navButtons, discord.NewSecondaryButton("◀️ Prev", "roles:prev"))
	}
	if end < len(p.allRoles) {
		navButtons = append(navButtons, discord.NewSecondaryButton("Next ▶️", "roles:next"))
	}
	navButtons = append(navButtons, discord.NewSecondaryButton("🔙 Back", "roles:back"))

	components = append(components, discord.NewActionRow(navButtons...))

	return interactivity.Message{
		Components: &components,
		Flags:      new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *ApplyPanel) viewSelectChannels() interactivity.Message {
	itemsPerPage := 25
	start := p.channelPage * itemsPerPage
	end := start + itemsPerPage
	if end > len(p.allChannels) {
		end = len(p.allChannels)
	}

	pageChannels := p.allChannels[start:end]
	totalPages := (len(p.allChannels) + itemsPerPage - 1) / itemsPerPage

	selectMenu := discord.NewChannelSelectMenu("channels:select", "Choose channels to protect...").
		WithMaxValues(25)

	components := []discord.LayoutComponent{
		discord.NewContainer(discord.NewTextDisplay("### 🛡️ Select Channels to Protect"), discord.NewTextDisplay(fmt.Sprintf("Page %d/%d — Select channels that should NOT be deleted.", p.channelPage+1, totalPages))).WithAccentColor(0x5865F2),
		discord.NewTextDisplay(fmt.Sprintf("### Page %d/%d — %d channels shown", p.channelPage+1, totalPages, len(pageChannels))),
		discord.NewActionRow(selectMenu),
	}

	var navButtons []discord.InteractiveComponent
	if p.channelPage > 0 {
		navButtons = append(navButtons, discord.NewSecondaryButton("◀️ Prev", "channels:prev"))
	}
	if end < len(p.allChannels) {
		navButtons = append(navButtons, discord.NewSecondaryButton("Next ▶️", "channels:next"))
	}
	navButtons = append(navButtons, discord.NewSecondaryButton("🔙 Back", "channels:back"))

	components = append(components, discord.NewActionRow(navButtons...))

	return interactivity.Message{
		Components: &components,
		Flags:      new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *ApplyPanel) viewConfirm() interactivity.Message {
	var summary string
	summary += fmt.Sprintf("**📋 Apply Summary**\n\n**Backup:** `%s`\n\n**Will restore:**\n", p.backupID)

	if p.restoreRoles {
		summary += fmt.Sprintf("✅ **Roles** (%d roles, %d protected)\n", len(p.data.Roles), len(p.protectedRoles))
	} else {
		summary += "❌ Roles (skipped)\n"
	}
	if p.restoreChannels {
		summary += fmt.Sprintf("✅ **Channels** (%d channels, %d protected)\n", len(p.data.Channels), len(p.protectedChannels))
	} else {
		summary += "❌ Channels (skipped)\n"
	}
	if p.restoreBans {
		summary += fmt.Sprintf("✅ **Bans** (%d bans)\n", len(p.data.Bans))
	} else {
		summary += "❌ Bans (skipped)\n"
	}

	summary += "\n**⚠️ This will delete and recreate channels and roles!**"

	return interactivity.Message{
		Components: &[]discord.LayoutComponent{
			discord.NewContainer(
				discord.NewTextDisplay("### ⚠️ Confirm Backup Apply"),
				discord.NewTextDisplay(summary),
			).WithAccentColor(0xFEE75C),
			discord.NewTextDisplay("### Are you sure you want to apply this backup?"),
			discord.NewActionRow(
				discord.NewSuccessButton("✅ Confirm", "confirm:yes"),
				discord.NewDangerButton("❌ Cancel", "confirm:no"),
			),
		},
		Flags: new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *ApplyPanel) viewApplying() interactivity.Message {
	components := []discord.LayoutComponent{
		discord.NewContainer(
			discord.NewTextDisplay("### ⏳ Applying Backup..."),
			discord.NewTextDisplay("Please wait while the backup is being applied. This may take a while..."),
		).WithAccentColor(0x5865F2),
		discord.NewTextDisplay("### Deleting existing channels and roles, then restoring from backup..."),
	}

	if p.restoreRoles {
		components = append(components, discord.NewTextDisplay("⏳ Restoring roles..."))
	}
	if p.restoreChannels {
		components = append(components, discord.NewTextDisplay("⏳ Restoring channels..."))
	}
	if p.restoreBans {
		components = append(components, discord.NewTextDisplay("⏳ Applying bans..."))
	}

	return interactivity.Message{
		Components: &components,
		Flags:      new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *ApplyPanel) viewDone() interactivity.Message {
	if p.result == nil {
		return p.viewDoneError()
	}

	summary := fmt.Sprintf("**✅ Backup Applied Successfully!**\n\n**Backup:** `%s`\n\n", p.backupID)

	if p.restoreRoles {
		summary += fmt.Sprintf("✅ %d roles created\n", p.result.RolesCreated)
	}
	if p.restoreChannels {
		summary += fmt.Sprintf("✅ %d channels created\n", p.result.ChannelsCreated)
	}
	if p.restoreBans {
		summary += fmt.Sprintf("✅ %d bans applied\n", p.result.BansApplied)
	}

	if len(p.result.Errors) > 0 {
		summary += fmt.Sprintf("\n**⚠️ %d errors:**\n", len(p.result.Errors))
		for i, err := range p.result.Errors {
			if i >= 5 {
				summary += fmt.Sprintf("... and %d more", len(p.result.Errors)-5)
				break
			}
			summary += fmt.Sprintf("• %s\n", err)
		}
	}

	return interactivity.Message{
		Components: &[]discord.LayoutComponent{
			discord.NewContainer(
				discord.NewTextDisplay("### ✅ Apply Complete"),
				discord.NewTextDisplay(summary),
			).WithAccentColor(0x57F287),
			discord.NewTextDisplay("*This panel will close automatically...*"),
		},
		Flags: new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *ApplyPanel) viewDoneError() interactivity.Message {
	embed := discord.Embed{
		Title:       "❌ Apply Failed",
		Description: "An error occurred while applying the backup.",
		Color:       0xED4245,
	}

	return interactivity.Message{
		Embeds: &[]discord.Embed{embed},
	}
}
