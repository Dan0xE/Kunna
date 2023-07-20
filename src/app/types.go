package main

type FileHash struct {
	FileName string `json:"path"`
	Hash     string `json:"hash"`
}

type Configuration struct {
	GitlabInstanceUrl       string `json:"gitlab_instance_url"`
	BunnyCDNApiUrl          string `json:"bunnycdn_api_url"`
	BunnyCDNStorageUrl      string `json:"bunnycdn_storage_url"`
	BunnyCDNStoragePullZone string `json:"bunnycdn_storage_pull_zone"`
	GitLabAPIKey            string `json:"gitlab_api_key"`
	BunnyCDNAPIKey          string `json:"bunny_cdn_api_key"`
	EncryptionKey           string `json:"encryption_key"`
	TempStoragePath         string `json:"temp_storage_path"`
	DiscordWebHook          string `json:"discord_webhook_url"`
}

type DiscordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []DiscordEmbedField `json:"fields"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type DiscordWebhookBody struct {
	Content string         `json:"content"`
	Embeds  []DiscordEmbed `json:"embeds"`
}

type GitLabRepo struct {
	ID                   int    `json:"id"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	WebURL               string `json:"web_url"`
	AvatarURL            string `json:"avatar_url"`
	GitSSHURL            string `json:"ssh_url_to_repo"`
	GitHTTPURL           string `json:"http_url_to_repo"`
	PathWithNamespace    string `json:"path_with_namespace"`
	DefaultBranch        string `json:"default_branch"`
	Visibility           string `json:"visibility"`
	IssuesEnabled        bool   `json:"issues_enabled"`
	MergeRequestsEnabled bool   `json:"merge_requests_enabled"`
	JobsEnabled          bool   `json:"jobs_enabled"`
	WikiEnabled          bool   `json:"wiki_enabled"`
	CreatedAt            string `json:"created_at"`
	LastActivityAt       string `json:"last_activity_at"`
}

type SyncConfig struct {
	Type string `json:"type"`
	Sync bool   `json:"sync"`
}
