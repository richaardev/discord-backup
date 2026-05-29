package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/richaardev/concord/interactivity"
	"github.com/richaardev/discord-backuper/internal/database/sqlc"
)

type createState int

const (
	createStateIdle createState = iota
	createStateBackingUp
	createStateSaving
	createStateCompleted
)

type CreatePanel struct {
	state     createState
	backupID  string
	guildID   snowflake.ID
	createdBy string
	rest      rest.Rest
	q         *sqlc.Queries
	data      *BackupData
	err       error
}

func NewCreatePanel(backupID string, guildID snowflake.ID, createdBy string, rest rest.Rest, q *sqlc.Queries) *CreatePanel {
	return &CreatePanel{
		state:     createStateIdle,
		backupID:  backupID,
		guildID:   guildID,
		createdBy: createdBy,
		rest:      rest,
		q:         q,
	}
}

func (p *CreatePanel) Init() {}

func (p *CreatePanel) Update(e any) (interactivity.PanelModel, interactivity.Cmd) {
	switch evt := e.(type) {
	case *events.ComponentInteractionCreate:
		customID := interactivity.GetCustomID(evt.Data.CustomID())
		switch customID {
		case "create:cancel":
			return nil, nil
		}
	}
	return p, nil
}

func (p *CreatePanel) View() interactivity.Message {
	switch p.state {
	case createStateIdle:
		return p.viewIdle()
	case createStateBackingUp:
		return p.viewBackingUp()
	case createStateSaving:
		return p.viewSaving()
	case createStateCompleted:
		return p.viewCompleted()
	}
	return interactivity.Message{}
}

func (p *CreatePanel) viewIdle() interactivity.Message {
	go p.run()

	return interactivity.Message{
		Components: &[]discord.LayoutComponent{
			discord.NewContainer(
				discord.NewTextDisplay("### 📦 Create Backup"),
				discord.NewTextDisplay(fmt.Sprintf("Ready to create backup `%s`.", p.backupID)),
			).WithAccentColor(0x5865F2),
			discord.NewTextDisplay("⏳ Preparing..."),
		},
		Flags: new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *CreatePanel) viewBackingUp() interactivity.Message {
	return interactivity.Message{
		Components: &[]discord.LayoutComponent{
			discord.NewContainer(discord.NewTextDisplay("### 📦 Creating Backup..."), discord.NewTextDisplay("🔍 Fetching server data...")).WithAccentColor(0x5865F2),
			discord.NewTextDisplay("⏳ Fetching channels, roles, and bans..."),
		},
		Flags: new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *CreatePanel) viewSaving() interactivity.Message {
	return interactivity.Message{
		Components: &[]discord.LayoutComponent{
			discord.NewContainer(discord.NewTextDisplay("### 📦 Creating Backup..."), discord.NewTextDisplay("💾 Saving to database...")).WithAccentColor(0x5865F2),
			discord.NewTextDisplay("⏳ Saving backup data..."),
		},
		Flags: new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *CreatePanel) viewCompleted() interactivity.Message {
	if p.err != nil {
		return interactivity.Message{
			Components: &[]discord.LayoutComponent{
				discord.NewContainer(discord.NewTextDisplay("### ❌ Backup Failed"), discord.NewTextDisplay(fmt.Sprintf("Error creating backup: %v", p.err))).WithAccentColor(0xED4245),
				discord.NewTextDisplay("*This panel will close automatically...*"),
			},
			Flags: new(discord.MessageFlagIsComponentsV2),
		}
	}

	return interactivity.Message{
		Components: &[]discord.LayoutComponent{
			discord.NewContainer(
				discord.NewTextDisplay("### ✅ Backup Created!"),
				discord.NewTextDisplay(fmt.Sprintf("Backup `%s` has been created successfully.", p.backupID)),
				discord.NewTextDisplay(fmt.Sprintf("**Roles:** %d", len(p.data.Roles))),
				discord.NewTextDisplay(fmt.Sprintf("**Channels:** %d", len(p.data.Channels))),
				discord.NewTextDisplay(fmt.Sprintf("**Bans:** %d", len(p.data.Bans))),
			).WithAccentColor(0x57F287),
			discord.NewTextDisplay("*This panel will close automatically...*"),
		},
		Flags: new(discord.MessageFlagIsComponentsV2),
	}
}

func (p *CreatePanel) run() {
	p.state = createStateBackingUp
	time.Sleep(500 * time.Millisecond)

	data, err := Create(p.guildID, p.rest)
	if err != nil {
		p.err = err
		p.state = createStateCompleted
		time.Sleep(5 * time.Second)
		return
	}
	p.data = data

	p.state = createStateSaving
	time.Sleep(500 * time.Millisecond)

	raw, err := json.Marshal(data)
	if err != nil {
		p.err = fmt.Errorf("failed to marshal backup: %w", err)
		p.state = createStateCompleted
		time.Sleep(5 * time.Second)
		return
	}

	if _, err := p.q.CreateBackup(context.Background(), sqlc.CreateBackupParams{
		ID:        p.backupID,
		GuildID:   p.guildID.String(),
		Data:      string(raw),
		CreatedBy: p.createdBy,
	}); err != nil {
		p.err = fmt.Errorf("failed to save backup: %w", err)
		p.state = createStateCompleted
		time.Sleep(5 * time.Second)
		return
	}

	p.state = createStateCompleted
	time.Sleep(5 * time.Second)
}
