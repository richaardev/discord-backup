package backup

import (
	"fmt"
	"log/slog"
	"strings"
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

	result *ApplyResult
}

func NewApplyPanel(backupID string, data *BackupData, guildID snowflake.ID, rest rest.Rest) *ApplyPanel {
	return &ApplyPanel{
		state:           stateConfigMain,
		backupID:        backupID,
		data:            data,
		guildID:         guildID,
		rest:            rest,
		restoreRoles:    true,
		restoreChannels: true,
		restoreBans:     true,
	}
}

func (p *ApplyPanel) Init() {}

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
	case "apply:start":
		p.state = stateConfirm
	case "cancel":
		return nil, nil
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

	cfg := ApplyConfig{
		RestoreRoles:    p.restoreRoles,
		RestoreChannels: p.restoreChannels,
		RestoreBans:     p.restoreBans,
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
		discord.NewActionRow(
			discord.NewSuccessButton("▶️ Apply Backup", "apply:start"),
			discord.NewDangerButton("❌ Cancel", "cancel"),
		),
	}

	return interactivity.Message{
		Components: &components,
		Flags:      new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *ApplyPanel) viewConfirm() interactivity.Message {
	var summary string
	summary += fmt.Sprintf("**📋 Apply Summary**\n\n**Backup:** `%s`\n\n**Will restore:**\n", p.backupID)

	if p.restoreRoles {
		summary += fmt.Sprintf("✅ **Roles** (%d roles)\n", len(p.data.Roles))
	} else {
		summary += "❌ Roles (skipped)\n"
	}
	if p.restoreChannels {
		summary += fmt.Sprintf("✅ **Channels** (%d channels)\n", len(p.data.Channels))
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

	var summary strings.Builder
	fmt.Fprintf(&summary, "**✅ Backup Applied Successfully!**\n\n**Backup:** `%s`\n\n", p.backupID)

	if p.restoreRoles {
		fmt.Fprintf(&summary, "✅ %d roles created\n", p.result.RolesCreated)
	}
	if p.restoreChannels {
		fmt.Fprintf(&summary, "✅ %d channels created\n", p.result.ChannelsCreated)
	}
	if p.restoreBans {
		fmt.Fprintf(&summary, "✅ %d bans applied\n", p.result.BansApplied)
	}

	if len(p.result.Errors) > 0 {
		fmt.Fprintf(&summary, "\n**⚠️ %d errors:**\n", len(p.result.Errors))
		for i, err := range p.result.Errors {
			if i >= 5 {
				fmt.Fprintf(&summary, "... and %d more", len(p.result.Errors)-5)
				break
			}
			fmt.Fprintf(&summary, "• %s\n", err)
		}
	}

	return interactivity.Message{
		Components: &[]discord.LayoutComponent{
			discord.NewContainer(
				discord.NewTextDisplay("### ✅ Apply Complete"),
				discord.NewTextDisplay(summary.String()),
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
