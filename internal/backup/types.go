package backup

type BackupData struct {
	Version  int             `json:"version"`
	GuildID  string          `json:"guild_id"`
	Roles    []RoleBackup    `json:"roles"`
	Channels []ChannelBackup `json:"channels"`
	Bans     []BanBackup     `json:"bans"`
}

type RoleBackup struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Hoist       bool   `json:"hoist"`
	Position    int    `json:"position"`
	Permissions int64  `json:"permissions"`
	Mentionable bool   `json:"mentionable"`
	Managed     bool   `json:"managed"`
	Icon        string `json:"icon,omitempty"`
	Emoji       string `json:"emoji,omitempty"`
}

type PermissionOverwriteBackup struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Allow int64  `json:"allow"`
	Deny  int64  `json:"deny"`
}

type ChannelBackup struct {
	ID                   string                       `json:"id"`
	Type                 int                          `json:"type"`
	Name                 string                       `json:"name"`
	Position             int                          `json:"position"`
	ParentID             string                       `json:"parent_id,omitempty"`
	Topic                string                       `json:"topic,omitempty"`
	NSFW                 bool                         `json:"nsfw,omitempty"`
	RateLimitPerUser     int                          `json:"rate_limit_per_user,omitempty"`
	Bitrate              int                          `json:"bitrate,omitempty"`
	UserLimit            int                          `json:"user_limit,omitempty"`
	PermissionOverwrites []PermissionOverwriteBackup  `json:"permission_overwrites,omitempty"`
}

func ptr[T any](v T) *T { return &v }

type BanBackup struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Reason   string `json:"reason,omitempty"`
}
