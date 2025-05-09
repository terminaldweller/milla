package main

import (
	"context"
	"log"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcdole/gofeed"
	lua "github.com/yuin/gopher-lua"
)

type LogModel struct {
	// Id        int64       `db:"id"`
	// Channel   string      `db:"channel"`
	Log string `db:"log"`
	// Nick      string      `db:"nick"`
	// DateAdded pgtype.Date `db:"dateadded"`
}

type PluginType struct {
	Name    string     `toml:"name"`
	Path    string     `toml:"path"`
	EnvVars [][]string `toml:"envVars"`
}

type CustomCommand struct {
	SQL          string   `toml:"sql"`
	Limit        int      `toml:"limit"`
	Context      []string `toml:"context"`
	Prompt       string   `toml:"prompt"`
	SystemPrompt string   `toml:"systemPrompt"`
}

type LuaLstates struct {
	LuaState *lua.LState
	Cancel   context.CancelFunc
}

type WatchList struct {
	AlertChannel []string   `toml:"alertChannel"`
	WatchList    [][]string `toml:"watchList"`
	WatchFiles   []string   `toml:"watchFiles"`
	Words        []string   `toml:"watchWords"`
	EventTypes   []string   `toml:"eventTypes"`
	FGColor      int        `toml:"fgColor"`
	BGColor      int        `toml:"bgColor"`
}

type LuaCommand struct {
	Path     string
	FuncName string
}

type TriggeredScripts struct {
	Path         string
	FuncName     string
	Channels     [][]string
	TriggerTypes []string
}

type RssFile struct {
	RssFile string   `toml:"rssFile"`
	Channel []string `toml:"channel"`
}

type TomlConfig struct {
	IrcServer                     string                   `toml:"ircServer"`
	IrcNick                       string                   `toml:"ircNick"`
	IrcSaslUser                   string                   `toml:"ircSaslUser"`
	IrcSaslPass                   string                   `toml:"ircSaslPass"`
	Endpoint                      string                   `toml:"endpoint"`
	Model                         string                   `toml:"model"`
	ChromaStyle                   string                   `toml:"chromaStyle"`
	ChromaFormatter               string                   `toml:"chromaFormatter"`
	Provider                      string                   `toml:"provider"`
	Apikey                        string                   `toml:"apikey"`
	ClientCertPath                string                   `toml:"clientCertPath"`
	ServerPass                    string                   `toml:"serverPass"`
	Bind                          string                   `toml:"bind"`
	Name                          string                   `toml:"name"`
	DatabaseAddress               string                   `toml:"databaseAddress"`
	DatabasePassword              string                   `toml:"databasePassword"`
	DatabaseUser                  string                   `toml:"databaseUser"`
	DatabaseName                  string                   `toml:"databaseName"`
	LLMProxy                      string                   `toml:"llmProxy"`
	IRCProxy                      string                   `toml:"ircProxy"`
	GeneralProxy                  string                   `toml:"generalProxy"`
	IRCDName                      string                   `toml:"ircdName"`
	WebIRCPassword                string                   `toml:"webIRCPassword"`
	WebIRCGateway                 string                   `toml:"webIRCGateway"`
	WebIRCHostname                string                   `toml:"webIRCHostname"`
	WebIRCAddress                 string                   `toml:"webIRCAddress"`
	RSSFile                       string                   `toml:"rssFile"`
	AnthropicVersion              string                   `toml:"anthropicVersion"`
	Plugins                       []string                 `toml:"plugins"`
	Context                       []string                 `toml:"context"`
	SystemPrompt                  string                   `toml:"systemPrompt"`
	CustomCommands                map[string]CustomCommand `toml:"customCommands"`
	WatchLists                    map[string]WatchList     `toml:"watchList"`
	LuaStates                     map[string]LuaLstates
	LuaCommands                   map[string]LuaCommand
	TriggeredScripts              map[string]TriggeredScripts
	Rss                           map[string]RssFile          `toml:"rss"`
	UserAgentActions              map[string]UserAgentRequest `toml:"userAgentActions"`
	Aliases                       map[string]Alias            `toml:"aliases"`
	RequestTimeout                int                         `toml:"requestTimeout"`
	MillaReconnectDelay           int                         `toml:"millaReconnectDelay"`
	IrcPort                       int                         `toml:"ircPort"`
	KeepAlive                     int                         `toml:"keepAlive"`
	MemoryLimit                   int                         `toml:"memoryLimit"`
	PingDelay                     int                         `toml:"pingDelay"`
	PingTimeout                   int                         `toml:"pingTimeout"`
	OllamaMirostat                int                         `json:"ollamaMirostat"`
	OllamaMirostatEta             float64                     `json:"ollamaMirostatEta"`
	OllamaMirostatTau             float64                     `json:"ollamaMirostatTau"`
	OllamaNumCtx                  int                         `json:"ollamaNumCtx"`
	OllamaRepeatLastN             int                         `json:"ollamaRepeatLastN"`
	OllamaRepeatPenalty           float64                     `json:"ollamaRepeatPenalty"`
	Temperature                   float64                     `json:"temperature"`
	OllamaSeed                    int                         `json:"ollamaSeed"`
	OllamaNumPredict              int                         `json:"ollamaNumPredict"`
	OllamaMinP                    float64                     `json:"ollamaMinP"`
	TopP                          float32                     `toml:"topP"`
	TopK                          int32                       `toml:"topK"`
	IrcBackOffInitialInterval     int                         `toml:"ircBackOffInitialInterval"`
	IrcBackOffRandomizationFactor float64                     `toml:"ircbackOffRandomizationFactor"`
	IrcBackOffMultiplier          float64                     `toml:"ircBackOffMultiplier"`
	IrcBackOffMaxInterval         int                         `toml:"ircBackOffMaxInterval"`
	DbBackOffInitialInterval      int                         `toml:"dbBackOffInitialInterval"`
	DbBackOffRandomizationFactor  float64                     `toml:"dbBackOffRandomizationFactor"`
	DbBackOffMultiplier           float64                     `toml:"dbBackOffMultiplier"`
	DbBackOffMaxInterval          int                         `toml:"dbBackOffMaxInterval"`
	EnableSasl                    bool                        `toml:"enableSasl"`
	SkipTLSVerify                 bool                        `toml:"skipTLSVerify"`
	UseTLS                        bool                        `toml:"useTLS"`
	DisableSTSFallback            bool                        `toml:"disableSTSFallback"`
	AllowFlood                    bool                        `toml:"allowFlood"`
	Debug                         bool                        `toml:"debug"`
	Out                           bool                        `toml:"out"`
	AdminOnly                     bool                        `toml:"adminOnly"`
	pool                          *pgxpool.Pool
	Admins                        []string   `toml:"admins"`
	IrcChannels                   [][]string `toml:"ircChannels"`
	ScrapeChannels                [][]string `toml:"scrapeChannels"`
}

func (config *TomlConfig) insertLState(
	name string,
	luaState *lua.LState,
	cancel context.CancelFunc,
) {
	if config.LuaStates == nil {
		config.LuaStates = make(map[string]LuaLstates)
	}
	config.LuaStates[name] = LuaLstates{
		LuaState: luaState,
		Cancel:   cancel,
	}
}

func (config *TomlConfig) deleteLstate(name string) {
	if config.LuaStates == nil {
		return
	}

	if config.LuaStates[name].Cancel != nil {
		config.LuaStates[name].Cancel()
	}
	delete(config.LuaStates, name)
}

func (config *TomlConfig) insertLuaCommand(
	cmd, path, name string,
) {
	if config.LuaCommands == nil {
		config.LuaCommands = make(map[string]LuaCommand)
	}
	config.LuaCommands[cmd] = LuaCommand{Path: path, FuncName: name}
}

func (config *TomlConfig) deleteLuaCommand(name string) {
	if config.LuaCommands == nil {
		return
	}
	delete(config.LuaCommands, name)
}

func (config *TomlConfig) insertTriggeredScript(path, cmd string, triggerType []string) {
	if config.TriggeredScripts == nil {
		config.TriggeredScripts = make(map[string]TriggeredScripts)
	}
	config.TriggeredScripts[path] = TriggeredScripts{
		Path:         path,
		FuncName:     cmd,
		TriggerTypes: triggerType,
	}
}

func (config *TomlConfig) deleteTriggeredScript(name string) {
	if config.TriggeredScripts == nil {
		return
	}

	delete(config.TriggeredScripts, name)
}

type AppConfig struct {
	Ircd map[string]TomlConfig `toml:"ircd"`
}

type OllamaRequestOptions struct {
	Mirostat      int     `json:"mirostat"`
	MirostatEta   float64 `json:"mirostat_eta"`
	MirostatTau   float64 `json:"mirostat_tau"`
	NumCtx        int     `json:"num_ctx"`
	RepeatLastN   int     `json:"repeat_last_n"`
	RepeatPenalty float64 `json:"repeat_penalty"`
	Temperature   float64 `json:"temperature"`
	Seed          int     `json:"seed"`
	NumPredict    int     `json:"num_predict"`
	TopK          int32   `json:"top_k"`
	TopP          float32 `json:"top_p"`
	MinP          float64 `json:"min_p"`
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
	System    string               `json:"system"`
	Messages  []MemoryElement      `json:"messages"`
}

type ORMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Refusal string `json:"refusal"`
}

type ORChoice struct {
	FinishReason string    `json:"finish_reason"`
	Index        int       `json:"index"`
	Message      ORMessage `json:"message"`
}

type ORUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
type ORResponse struct {
	Id                string     `json:"id"`
	Provider          string     `json:"provider"`
	Model             string     `json:"model"`
	Object            string     `json:"object"`
	Created           int64      `json:"created"`
	Choices           []ORChoice `json:"choices"`
	SystemFingerprint string     `json:"system_fingerprint"`
	Usage             ORUsage    `json:"usage"`
}

type MemoryElement struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type FeedConfig struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	UserAgent  string `json:"userAgent"`
	Proxy      string `json:"proxy"`
	Timeout    int    `json:"timeout"`
	FeedParser *gofeed.Parser
}

type RSSConfig struct {
	Feeds  []FeedConfig `json:"feeds"`
	Period int          `json:"period"`
}

func LogError(err error) {
	fn, file, line, ok := runtime.Caller(1)
	if ok {
		log.Printf("%s: %s-%d >>> %v", runtime.FuncForPC(fn).Name(), file, line, err)
	} else {
		log.Print(err)
	}
}

func LogErrorFatal(err error) {
	fn, file, line, ok := runtime.Caller(1)
	if ok {
		log.Fatalf("%s: %s-%d >>> %v", runtime.FuncForPC(fn).Name(), file, line, err)
	} else {
		log.Fatal(err)
	}
}

type ProxyRoundTripper struct {
	APIKey string

	ProxyURL string
}

type UserAgentRequest struct {
	Agent_Name   string `json:"agent_name" toml:"agent_name"`
	Instructions string `json:"instructions" toml:"instructions"`
	Query        string `json:"query" toml:"query"`
}

type UserAgentResponse struct {
	Agent_Name string `json:"agent_name"`
	Response   string `json:"response"`
}

type Alias struct {
	Alias string `toml:"alias"`
}
