package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/google/generative-ai-go/genai"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lrstanley/girc"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/net/proxy"
	"google.golang.org/api/option"
)

var (
	errNotEnoughArgs     = errors.New("not enough arguments")
	errUnknCmd           = errors.New("unknown command")
	errUnknConfig        = errors.New("unknown config name")
	errCantSet           = errors.New("can't set field")
	errWrongDataForField = errors.New("wrong data type for field")
	errUnsupportedType   = errors.New("unsupported type")
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

func addSaneDefaults(config *TomlConfig) {
	if config.IrcNick == "" {
		config.IrcNick = "milla"
	}

	if config.ChromaStyle == "" {
		config.ChromaStyle = "rose-pine-moon"
	}

	if config.ChromaFormatter == "" {
		config.ChromaFormatter = "noop"
	}

	if config.DatabaseAddress == "" {
		config.DatabaseAddress = "postgres"
	}

	if config.DatabaseUser == "" {
		config.DatabaseUser = "milla"
	}

	if config.DatabaseName == "" {
		config.DatabaseName = "milladb"
	}

	if config.Temp == 0 {
		config.Temp = 0.5
	}

	if config.RequestTimeout == 0 {
		config.RequestTimeout = 10
	}

	if config.MillaReconnectDelay == 0 {
		config.MillaReconnectDelay = 30
	}

	if config.IrcPort == 0 {
		config.IrcPort = 6697
	}

	if config.KeepAlive == 0 {
		config.KeepAlive = 600
	}

	if config.MemoryLimit == 0 {
		config.MemoryLimit = 20
	}

	if config.PingDelay == 0 {
		config.PingDelay = 20
	}

	if config.PingTimeout == 0 {
		config.PingTimeout = 20
	}

	if config.TopP == 0.0 {
		config.TopP = 0.9
	}
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

func getTableFromChanName(channel, ircdName string) string {
	tableName := ircdName + "_" + channel
	tableName = strings.ReplaceAll(tableName, "#", "")
	tableName = strings.ReplaceAll(tableName, "-", "_")
	tableName = strings.TrimSpace(tableName)

	return tableName
}

func sanitizeLog(log string) string {
	sanitizeLog := strings.ReplaceAll(log, "'", " ")

	return sanitizeLog
}

func returnGeminiResponse(resp *genai.GenerateContentResponse) string {
	result := ""

	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				result += fmt.Sprintln(part)
			}
		}
	}

	return result
}

func extractLast256ColorEscapeCode(str string) (string, error) {
	pattern256F := `\033\[38;5;(\d+)m`
	// pattern256B := `\033\[48;5;(\d+)m`
	// pattern16mF := `\033\[38;2;(\d+);(\d+);(\d+)m`
	// pattern16mB := `\033\[48;2;(\d+);(\d+);(\d+)m`

	r, err := regexp.Compile(pattern256F)
	if err != nil {
		return "", fmt.Errorf("failed to compile regular expression: %w", err)
	}

	matches := r.FindAllStringSubmatch(str, -1)
	if len(matches) == 0 {
		return "", nil
	}

	lastMatch := matches[len(matches)-1]

	return lastMatch[1], nil
}

func chunker(inputString string, chromaFormatter string) []string {
	chunks := strings.Split(inputString, "\n")

	switch chromaFormatter {
	case "terminal":
		fallthrough
	case "terminal8":
		fallthrough
	case "terminal16":
		fallthrough
	case "terminal256":
		for count, chunk := range chunks {
			lastColorCode, err := extractLast256ColorEscapeCode(chunk)
			if err != nil {
				continue
			}

			if count <= len(chunks)-2 {
				chunks[count+1] = fmt.Sprintf("\033[38;5;%sm", lastColorCode) + chunks[count+1]
			}
		}
	case "terminal16m":
		fallthrough
	default:
	}

	return chunks
}

func sendToIRC(
	client *girc.Client,
	event girc.Event,
	message string,
	chromaFormatter string,
) {
	chunks := chunker(message, chromaFormatter)

	for _, chunk := range chunks {
		if len(strings.TrimSpace(chunk)) == 0 {
			continue
		}

		client.Cmd.Reply(event, chunk)
	}
}

func getHelpString() string {
	helpString := "Commands:\n"
	helpString += "help - show this help message\n"
	helpString += "set - set a configuration value\n"
	helpString += "get - get a configuration value\n"
	helpString += "join - joins a given channel\n"
	helpString += "leave - leaves a given channel\n"
	helpString += "cmd - run a custom command defined in the customcommands file\n"
	helpString += "getall - returns all config options with their value\n"
	helpString += "memstats - returns the memory status currently being used\n"

	return helpString
}

func setFieldByName(v reflect.Value, field string, value string) error {
	fieldValue := v.FieldByName(field)
	if !fieldValue.IsValid() {
		return errUnknConfig
	}

	if !fieldValue.CanSet() {
		return errCantSet
	}

	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Int:
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return errWrongDataForField
		}

		fieldValue.SetInt(int64(intValue))
	case reflect.Float64:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return errWrongDataForField
		}

		fieldValue.SetFloat(floatValue)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return errWrongDataForField
		}

		fieldValue.SetBool(boolValue)
	default:
		return errUnsupportedType
	}

	return nil
}

func byteToMByte(bytes uint64,
) uint64 {
	return bytes / 1024 / 1024
}

func handleCustomCommand(
	args []string,
	client *girc.Client,
	event girc.Event,
	appConfig *TomlConfig,
) {
	log.Println(args)
	if len(args) < 2 {
		client.Cmd.Reply(event, errNotEnoughArgs.Error())

		return
	}

	customCommand := appConfig.CustomCommands[args[1]]

	if customCommand.SQL == "" {
		client.Cmd.Reply(event, "empty sql commands in the custom command")

		return
	}

	if appConfig.pool == nil {
		client.Cmd.Reply(event, "no database connection")

		return
	}

	log.Println(customCommand.SQL)

	rows, err := appConfig.pool.Query(context.Background(), customCommand.SQL)
	if err != nil {
		client.Cmd.Reply(event, "error: "+err.Error())

		return
	}
	defer rows.Close()

	logs, err := pgx.CollectRows(rows, pgx.RowToStructByName[LogModel])
	if err != nil {
		log.Println(err.Error())

		return
	}

	log.Println(logs)
	logs = logs[:customCommand.Limit]

	if err != nil {
		log.Println(err.Error())

		return
	}

	switch appConfig.Provider {
	case "chatgpt":
		var gptMemory []openai.ChatCompletionMessage

		for _, log := range logs {
			gptMemory = append(gptMemory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: log.Log,
			})
		}

		chatGPTRequest(appConfig, client, event, &gptMemory, customCommand.Prompt)
	case "gemini":
		var geminiMemory []*genai.Content

		for _, log := range logs {
			geminiMemory = append(geminiMemory, &genai.Content{
				Parts: []genai.Part{
					genai.Text(log.Log),
				},
				Role: "user",
			})
		}

		geminiRequest(appConfig, client, event, &geminiMemory, customCommand.Prompt)
	case "ollama":
		var ollamaMemory []MemoryElement

		for _, log := range logs {
			ollamaMemory = append(ollamaMemory, MemoryElement{
				Role:    "user",
				Content: log.Log,
			})
		}

		ollamaRequest(appConfig, client, event, &ollamaMemory, customCommand.Prompt)
	default:
	}
}

func isFromAdmin(admins []string, event girc.Event) bool {
	messageFromAdmin := false

	for _, admin := range admins {
		if event.Source.Name == admin {
			messageFromAdmin = true

			break
		}
	}

	return messageFromAdmin
}

func runCommand(
	client *girc.Client,
	event girc.Event,
	appConfig *TomlConfig,
) {
	cmd := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimPrefix(cmd, "/")
	args := strings.Split(cmd, " ")

	if appConfig.AdminOnly && !isFromAdmin(appConfig.Admins, event) {
		return
	}

	switch args[0] {
	case "help":
		sendToIRC(client, event, getHelpString(), "noop")
	case "set":
		if len(args) < 3 {
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		err := setFieldByName(reflect.ValueOf(appConfig).Elem(), args[1], args[2])
		if err != nil {
			client.Cmd.Reply(event, err.Error())
		}
	case "get":
		if len(args) < 2 {
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		log.Println(args[1])

		v := reflect.ValueOf(*appConfig)
		field := v.FieldByName(args[1])

		if !field.IsValid() {
			client.Cmd.Reply(event, errUnknConfig.Error())

			break
		}

		client.Cmd.Reply(event, fmt.Sprintf("%v", field.Interface()))
	case "getall":
		v := reflect.ValueOf(*appConfig)
		t := v.Type()

		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			fieldValue := v.Field(i).Interface()
			client.Cmd.Reply(event, fmt.Sprintf("%s: %v", field.Name, fieldValue))
		}
	case "memstats":
		var memStats runtime.MemStats

		runtime.ReadMemStats(&memStats)

		client.Cmd.Reply(event, fmt.Sprintf("Alloc: %d MiB", byteToMByte(memStats.Alloc)))
		client.Cmd.Reply(event, fmt.Sprintf("TotalAlloc: %d MiB", byteToMByte(memStats.TotalAlloc)))
		client.Cmd.Reply(event, fmt.Sprintf("Sys: %d MiB", byteToMByte(memStats.Sys)))
	case "join":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		if len(args) < 2 {
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		client.Cmd.Join(args[1])
	case "leave":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		if len(args) < 2 {
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		client.Cmd.Part(args[1])
	case "cmd":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		handleCustomCommand(args, client, event, appConfig)
	default:
		client.Cmd.Reply(event, errUnknCmd.Error())
	}
}

func doOllamaRequest(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	ollamaMemory *[]MemoryElement,
	prompt string,
) (*http.Response, error) {
	var jsonPayload []byte

	var err error

	memoryElement := MemoryElement{
		Role:    "user",
		Content: prompt,
	}

	if len(*ollamaMemory) > appConfig.MemoryLimit {
		*ollamaMemory = []MemoryElement{}
	}

	*ollamaMemory = append(*ollamaMemory, memoryElement)

	ollamaRequest := OllamaChatRequest{
		Model:     appConfig.Model,
		KeepAlive: time.Duration(appConfig.KeepAlive),
		Stream:    false,
		Messages:  *ollamaMemory,
		Options: OllamaRequestOptions{
			Temperature: appConfig.Temp,
		},
	}

	jsonPayload, err = json.Marshal(ollamaRequest)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return nil, fmt.Errorf("could not marshal json payload: %v", err)
	}

	log.Printf("json payload: %s", string(jsonPayload))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	request, err := http.NewRequest(http.MethodPost, appConfig.OllamaEndpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return nil, fmt.Errorf("could not make a new http request: %v", err)
	}

	request = request.WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")

	var httpClient http.Client

	var dialer proxy.Dialer

	if appConfig.LLMProxy != "" {
		proxyURL, err := url.Parse(appConfig.IRCProxy)
		if err != nil {
			cancel()

			log.Fatal(err.Error())
		}

		dialer, err = proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
		if err != nil {
			cancel()

			log.Fatal(err.Error())
		}

		httpClient = http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}
	}

	return httpClient.Do(request)
}

func ollamaRequest(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	ollamaMemory *[]MemoryElement,
	prompt string,
) {
	response, err := doOllamaRequest(appConfig, client, event, ollamaMemory, prompt)

	if response == nil {
		return
	}

	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return
	}

	defer response.Body.Close()

	log.Println("response body:", response.Body)

	var writer bytes.Buffer

	var ollamaChatResponse OllamaChatMessagesResponse

	err = json.NewDecoder(response.Body).Decode(&ollamaChatResponse)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())
	}

	assistantElement := MemoryElement{
		Role:    "assistant",
		Content: ollamaChatResponse.Messages.Content,
	}

	*ollamaMemory = append(*ollamaMemory, assistantElement)

	log.Println(ollamaChatResponse)

	err = quick.Highlight(&writer,
		ollamaChatResponse.Messages.Content,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return
	}

	sendToIRC(client, event, writer.String(), appConfig.ChromaFormatter)
}

func ollamaHandler(
	irc *girc.Client,
	appConfig *TomlConfig,
	ollamaMemory *[]MemoryElement,
) {
	irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
		if !strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
			return
		}

		if appConfig.AdminOnly {
			byAdmin := false

			for _, admin := range appConfig.Admins {
				if event.Source.Name == admin {
					byAdmin = true
				}
			}

			if !byAdmin {
				return
			}
		}

		prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		log.Println(prompt)

		if string(prompt[0]) == "/" {
			runCommand(client, event, appConfig)

			return
		}

		ollamaRequest(appConfig, client, event, ollamaMemory, prompt)
	})
}

func doGeminiRequest(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	geminiMemory *[]*genai.Content,
	prompt string,
) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	clientGemini, err := genai.NewClient(ctx, option.WithAPIKey(appConfig.Apikey))
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}
	defer clientGemini.Close()

	model := clientGemini.GenerativeModel(appConfig.Model)
	model.SetTemperature(float32(appConfig.Temp))
	model.SetTopK(appConfig.TopK)
	model.SetTopP(appConfig.TopP)

	cs := model.StartChat()

	cs.History = *geminiMemory

	resp, err := cs.SendMessage(ctx, genai.Text(prompt))
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	return returnGeminiResponse(resp)
}

func geminiRequest(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	geminiMemory *[]*genai.Content,
	prompt string,
) {
	geminiResponse := doGeminiRequest(appConfig, client, event, geminiMemory, prompt)
	log.Println(geminiResponse)

	if len(*geminiMemory) > appConfig.MemoryLimit {
		*geminiMemory = []*genai.Content{}
	}

	*geminiMemory = append(*geminiMemory, &genai.Content{
		Parts: []genai.Part{
			genai.Text(prompt),
		},
		Role: "user",
	})

	*geminiMemory = append(*geminiMemory, &genai.Content{
		Parts: []genai.Part{
			genai.Text(geminiResponse),
		},
		Role: "model",
	})

	var writer bytes.Buffer

	err := quick.Highlight(
		&writer,
		geminiResponse,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return
	}

	sendToIRC(client, event, writer.String(), appConfig.ChromaFormatter)
}

func geminiHandler(
	irc *girc.Client,
	appConfig *TomlConfig,
	geminiMemory *[]*genai.Content,
) {
	irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
		if !strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
			return
		}

		if appConfig.AdminOnly {
			byAdmin := false

			for _, admin := range appConfig.Admins {
				if event.Source.Name == admin {
					byAdmin = true
				}
			}

			if !byAdmin {
				return
			}
		}

		prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		log.Println(prompt)

		if string(prompt[0]) == "/" {
			runCommand(client, event, appConfig)

			return
		}

		geminiRequest(appConfig, client, event, geminiMemory, prompt)
	})
}

func doChatGPTRequest(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	gptMemory *[]openai.ChatCompletionMessage,
	prompt string,
) (openai.ChatCompletionResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	var httpClient http.Client

	if appConfig.LLMProxy != "" {
		proxyURL, err := url.Parse(appConfig.IRCProxy)
		if err != nil {
			cancel()
			client.Cmd.ReplyTo(event, "error: "+err.Error())

			log.Fatal(err.Error())
		}

		dialer, err := proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
		if err != nil {
			cancel()
			client.Cmd.ReplyTo(event, "error: "+err.Error())

			log.Fatal(err.Error())
		}

		httpClient = http.Client{
			Transport: &http.Transport{
				Dial: dialer.Dial,
			},
		}
	}

	config := openai.DefaultConfig(appConfig.Apikey)
	config.HTTPClient = &httpClient

	gptClient := openai.NewClientWithConfig(config)

	*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	})

	resp, err := gptClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    appConfig.Model,
		Messages: *gptMemory,
	})

	return resp, err
}

func chatGPTRequest(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	gptMemory *[]openai.ChatCompletionMessage,
	prompt string,
) {
	resp, err := doChatGPTRequest(appConfig, client, event, gptMemory, prompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return
	}

	*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp.Choices[0].Message.Content,
	})

	if len(*gptMemory) > appConfig.MemoryLimit {
		*gptMemory = []openai.ChatCompletionMessage{}
	}

	var writer bytes.Buffer

	err = quick.Highlight(
		&writer,
		resp.Choices[0].Message.Content,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return
	}

	sendToIRC(client, event, writer.String(), appConfig.ChromaFormatter)
}

func chatGPTHandler(
	irc *girc.Client,
	appConfig *TomlConfig,
	gptMemory *[]openai.ChatCompletionMessage,
) {
	irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
		if !strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
			return
		}

		if appConfig.AdminOnly {
			byAdmin := false

			for _, admin := range appConfig.Admins {
				if event.Source.Name == admin {
					byAdmin = true
				}
			}

			if !byAdmin {
				return
			}
		}

		prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		log.Println(prompt)

		if string(prompt[0]) == "/" {
			runCommand(client, event, appConfig)

			return
		}

		chatGPTRequest(appConfig, client, event, gptMemory, prompt)
	})
}

func connectToDB(appConfig *TomlConfig, ctx *context.Context, poolChan chan *pgxpool.Pool) {
	for {
		dbURL := fmt.Sprintf(
			"postgres://%s:%s@%s/%s",
			appConfig.DatabaseUser,
			appConfig.DatabasePassword,
			appConfig.DatabaseAddress,
			appConfig.DatabaseName)

		log.Println("dbURL:", dbURL)

		poolConfig, err := pgxpool.ParseConfig(dbURL)
		if err != nil {
			log.Println(err)
		}

		pool, err := pgxpool.NewWithConfig(*ctx, poolConfig)
		if err != nil {
			log.Println(err)
			time.Sleep(time.Duration(appConfig.MillaReconnectDelay) * time.Second)
		} else {
			log.Printf("%s connected to database", appConfig.IRCDName)

			for _, channel := range appConfig.ScrapeChannels {
				tableName := getTableFromChanName(channel, appConfig.IRCDName)
				query := fmt.Sprintf(
					`create table if not exists %s (
					id serial primary key,
					channel text not null,
					log text not null,
					nick text not null,
					dateadded timestamp default current_timestamp
				)`, tableName)

				_, err = pool.Exec(*ctx, query)
				if err != nil {
					log.Println(err.Error())
					time.Sleep(time.Duration(appConfig.MillaReconnectDelay) * time.Second)
				}
			}

			appConfig.pool = pool
			poolChan <- pool
		}
	}
}

func scrapeChannel(irc *girc.Client, poolChan chan *pgxpool.Pool, appConfig TomlConfig) {
	irc.Handlers.AddBg(girc.PRIVMSG, func(_ *girc.Client, event girc.Event) {
		pool := <-poolChan
		tableName := getTableFromChanName(event.Params[0], appConfig.IRCDName)
		query := fmt.Sprintf(
			"insert into %s (channel,log,nick) values ('%s','%s','%s')",
			tableName,
			sanitizeLog(event.Params[0]),
			event.Last(),
			event.Source.Name,
		)

		_, err := pool.Exec(context.Background(), query)
		if err != nil {
			log.Println(err.Error())
		}
	})
}

func runIRC(appConfig TomlConfig) {
	var OllamaMemory []MemoryElement

	var GeminiMemory []*genai.Content

	var GPTMemory []openai.ChatCompletionMessage

	poolChan := make(chan *pgxpool.Pool, 1)

	irc := girc.New(girc.Config{
		Server:             appConfig.IrcServer,
		Port:               appConfig.IrcPort,
		Nick:               appConfig.IrcNick,
		User:               appConfig.IrcNick,
		Name:               appConfig.IrcNick,
		SSL:                appConfig.UseTLS,
		PingDelay:          time.Duration(appConfig.PingDelay),
		PingTimeout:        time.Duration(appConfig.PingTimeout),
		AllowFlood:         appConfig.AllowFlood,
		DisableSTSFallback: appConfig.DisableSTSFallback,
		GlobalFormat:       true,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: appConfig.SkipTLSVerify,
			ServerName:         appConfig.IrcServer,
		},
	})

	if appConfig.WebIRCGateway != "" {
		irc.Config.WebIRC.Address = appConfig.WebIRCAddress
		irc.Config.WebIRC.Gateway = appConfig.WebIRCGateway
		irc.Config.WebIRC.Hostname = appConfig.WebIRCHostname
		irc.Config.WebIRC.Password = appConfig.WebIRCPassword
	}

	if appConfig.Debug {
		irc.Config.Debug = os.Stdout
	}

	if appConfig.Out {
		irc.Config.Out = os.Stdout
	}

	irc.Config.ServerPass = appConfig.ServerPass

	if appConfig.Bind != "" {
		irc.Config.Bind = appConfig.Bind
	}

	if appConfig.Name != "" {
		irc.Config.Name = appConfig.Name
	}

	if appConfig.EnableSasl && appConfig.IrcSaslPass != "" && appConfig.IrcSaslUser != "" {
		irc.Config.SASL = &girc.SASLPlain{
			User: appConfig.IrcSaslUser,
			Pass: appConfig.IrcSaslPass,
		}
	}

	if appConfig.EnableSasl && appConfig.ClientCertPath != "" {
		cert, err := tls.LoadX509KeyPair(appConfig.ClientCertPath, appConfig.ClientCertPath)
		if err != nil {
			log.Println("invalid client certificate.")

			return
		}

		irc.Config.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		for _, channel := range appConfig.IrcChannels {
			c.Cmd.Join(channel)
		}
	})

	switch appConfig.Provider {
	case "ollama":
		ollamaHandler(irc, &appConfig, &OllamaMemory)
	case "gemini":
		geminiHandler(irc, &appConfig, &GeminiMemory)
	case "chatgpt":
		chatGPTHandler(irc, &appConfig, &GPTMemory)
	}

	if appConfig.DatabaseAddress != "" {
		context, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
		defer cancel()

		go connectToDB(&appConfig, &context, poolChan)
	}

	if len(appConfig.ScrapeChannels) > 0 {
		irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
			for _, channel := range appConfig.ScrapeChannels {
				c.Cmd.Join(channel)
			}
		})

		go scrapeChannel(irc, poolChan, appConfig)
	}

	for {
		var dialer proxy.Dialer

		if appConfig.IRCProxy != "" {
			proxyURL, err := url.Parse(appConfig.IRCProxy)
			if err != nil {
				log.Fatal(err.Error())
			}

			dialer, err = proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
			if err != nil {
				log.Fatal(err.Error())
			}
		}

		if err := irc.DialerConnect(dialer); err != nil {
			log.Println(err)
			log.Println("reconnecting in " + strconv.Itoa(appConfig.MillaReconnectDelay))
			time.Sleep(time.Duration(appConfig.MillaReconnectDelay) * time.Second)
		} else {
			return
		}
	}
}

func main() {
	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)

	configPath := flag.String("config", "./config.toml", "path to the config file")

	flag.Parse()

	data, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	var config AppConfig

	_, err = toml.Decode(string(data), &config)
	if err != nil {
		log.Fatal(err)
	}

	for key, value := range config.Ircd {
		addSaneDefaults(&value)
		value.IRCDName = key
		config.Ircd[key] = value
	}

	for k, v := range config.Ircd {
		log.Println(k, v)
	}

	for _, v := range config.Ircd {
		go runIRC(v)
	}

	<-quitChannel
}
