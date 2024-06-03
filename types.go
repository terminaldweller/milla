package main

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LogModel struct {
	// Id        int64       `db:"id"`
	// Channel   string      `db:"channel"`
	Log string `db:"log"`
	// Nick      string      `db:"nick"`
	// DateAdded pgtype.Date `db:"dateadded"`
}

type CustomCommand struct {
	SQL    string `toml:"sql"`
	Limit  int    `toml:"limit"`
	Prompt string `toml:"prompt"`
}

type TomlConfig struct {
	IrcServer           string                   `toml:"ircServer"`
	IrcNick             string                   `toml:"ircNick"`
	IrcSaslUser         string                   `toml:"ircSaslUser"`
	IrcSaslPass         string                   `toml:"ircSaslPass"`
	OllamaEndpoint      string                   `toml:"ollamaEndpoint"`
	Model               string                   `toml:"model"`
	ChromaStyle         string                   `toml:"chromaStyle"`
	ChromaFormatter     string                   `toml:"chromaFormatter"`
	Provider            string                   `toml:"provider"`
	Apikey              string                   `toml:"apikey"`
	OllamaSystem        string                   `toml:"ollamaSystem"`
	ClientCertPath      string                   `toml:"clientCertPath"`
	ServerPass          string                   `toml:"serverPass"`
	Bind                string                   `toml:"bind"`
	Name                string                   `toml:"name"`
	DatabaseAddress     string                   `toml:"databaseAddress"`
	DatabasePassword    string                   `toml:"databasePassword"`
	DatabaseUser        string                   `toml:"databaseUser"`
	DatabaseName        string                   `toml:"databaseName"`
	LLMProxy            string                   `toml:"llmProxy"`
	IRCProxy            string                   `toml:"ircProxy"`
	IRCDName            string                   `toml:"ircdName"`
	WebIRCPassword      string                   `toml:"webIRCPassword"`
	WebIRCGateway       string                   `toml:"webIRCGateway"`
	WebIRCHostname      string                   `toml:"webIRCHostname"`
	WebIRCAddress       string                   `toml:"webIRCAddress"`
	Plugins             []string                 `toml:"plugins"`
	CustomCommands      map[string]CustomCommand `toml:"customCommands"`
	Temp                float64                  `toml:"temp"`
	RequestTimeout      int                      `toml:"requestTimeout"`
	MillaReconnectDelay int                      `toml:"millaReconnectDelay"`
	IrcPort             int                      `toml:"ircPort"`
	KeepAlive           int                      `toml:"keepAlive"`
	MemoryLimit         int                      `toml:"memoryLimit"`
	PingDelay           int                      `toml:"pingDelay"`
	PingTimeout         int                      `toml:"pingTimeout"`
	TopP                float32                  `toml:"topP"`
	TopK                int32                    `toml:"topK"`
	EnableSasl          bool                     `toml:"enableSasl"`
	SkipTLSVerify       bool                     `toml:"skipTLSVerify"`
	UseTLS              bool                     `toml:"useTLS"`
	DisableSTSFallback  bool                     `toml:"disableSTSFallback"`
	AllowFlood          bool                     `toml:"allowFlood"`
	Debug               bool                     `toml:"debug"`
	Out                 bool                     `toml:"out"`
	AdminOnly           bool                     `toml:"adminOnly"`
	pool                *pgxpool.Pool
	Admins              []string `toml:"admins"`
	IrcChannels         []string `toml:"ircChannels"`
	ScrapeChannels      []string `toml:"scrapeChannels"`
}

type AppConfig struct {
	Ircd map[string]TomlConfig `toml:"ircd"`
}

type OllamaRequestOptions struct {
	Temperature float64 `json:"temperature"`
}

type OllamaChatResponse struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaChatMessagesResponse struct {
	Messages OllamaChatResponse `json:"message"`
}

type OllamaChatRequest struct {
	Model     string               `json:"model"`
	Stream    bool                 `json:"stream"`
	KeepAlive time.Duration        `json:"keep_alive"`
	Options   OllamaRequestOptions `json:"options"`
	Messages  []MemoryElement      `json:"messages"`
}

type MemoryElement struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
