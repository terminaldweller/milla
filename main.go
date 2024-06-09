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
	return bytes / 1024 / 1024 //nolint: mnd,gomnd
}

func handleCustomCommand(
	args []string,
	client *girc.Client,
	event girc.Event,
	appConfig *TomlConfig,
) {
	log.Println(args)

	if len(args) < 2 { //nolint: mnd,gomnd
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

	if customCommand.Limit != 0 {
		logs = logs[:customCommand.Limit]
	}

	log.Println(logs)

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

		result := ChatGPTRequestProcessor(appConfig, client, event, &gptMemory, customCommand.Prompt)
		if result != "" {
			sendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
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

		result := GeminiRequestProcessor(appConfig, client, event, &geminiMemory, customCommand.Prompt)
		if result != "" {
			sendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	case "ollama":
		var ollamaMemory []MemoryElement

		for _, log := range logs {
			ollamaMemory = append(ollamaMemory, MemoryElement{
				Role:    "user",
				Content: log.Log,
			})
		}

		result := OllamaRequestProcessor(appConfig, client, event, &ollamaMemory, customCommand.Prompt)
		if result != "" {
			sendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
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
		if len(args) < 3 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		err := setFieldByName(reflect.ValueOf(appConfig).Elem(), args[1], args[2])
		if err != nil {
			client.Cmd.Reply(event, err.Error())
		}
	case "get":
		if len(args) < 2 { //nolint: mnd,gomnd
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
		value := reflect.ValueOf(*appConfig)
		t := value.Type()

		for i := range value.NumField() {
			field := t.Field(i)
			fieldValue := value.Field(i).Interface()
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

		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		client.Cmd.Join(args[1])
	case "leave":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		client.Cmd.Part(args[1])
	case "cmd":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		handleCustomCommand(args, client, event, appConfig)
	case "load":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		RunScript(args[1], client, appConfig)
	case "unload":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		appConfig.deleteLstate(args[1])
	default:
		client.Cmd.Reply(event, errUnknCmd.Error())
	}
}

func DoOllamaRequest(
	appConfig *TomlConfig,
	ollamaMemory *[]MemoryElement,
	prompt string,
) (string, error) {
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

		return "", err
	}

	log.Printf("json payload: %s", string(jsonPayload))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	request, err := http.NewRequest(http.MethodPost, appConfig.OllamaEndpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {

		return "", err
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
	response, err := httpClient.Do(request)

	if err != nil {
		return "", err
	}

	if err != nil {

		return "", err
	}

	defer response.Body.Close()

	log.Println("response body:", response.Body)

	var ollamaChatResponse OllamaChatMessagesResponse

	err = json.NewDecoder(response.Body).Decode(&ollamaChatResponse)
	if err != nil {
		return "", err
	}

	return ollamaChatResponse.Messages.Content, nil
}

func OllamaRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	ollamaMemory *[]MemoryElement,
	prompt string,
) string {
	response, err := DoOllamaRequest(appConfig, ollamaMemory, prompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	assistantElement := MemoryElement{
		Role:    "assistant",
		Content: response,
	}

	*ollamaMemory = append(*ollamaMemory, assistantElement)

	log.Println(response)

	var writer bytes.Buffer

	err = quick.Highlight(&writer,
		response,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	return writer.String()
}

func OllamaHandler(
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

		result := OllamaRequestProcessor(appConfig, client, event, ollamaMemory, prompt)
		if result != "" {
			sendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}

func DoGeminiRequest(
	appConfig *TomlConfig,
	geminiMemory *[]*genai.Content,
	prompt string,
) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	clientGemini, err := genai.NewClient(ctx, option.WithAPIKey(appConfig.Apikey))
	if err != nil {

		return "", err
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

		return "", err
	}

	return returnGeminiResponse(resp), nil
}

func GeminiRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	geminiMemory *[]*genai.Content,
	prompt string,
) string {
	geminiResponse, err := DoGeminiRequest(appConfig, geminiMemory, prompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

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

	err = quick.Highlight(
		&writer,
		geminiResponse,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	return writer.String()
}

func GeminiHandler(
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

		result := GeminiRequestProcessor(appConfig, client, event, geminiMemory, prompt)

		if result != "" {
			sendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}

func DoChatGPTRequest(
	appConfig *TomlConfig,
	gptMemory *[]openai.ChatCompletionMessage,
	prompt string,
) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	var httpClient http.Client

	if appConfig.LLMProxy != "" {
		proxyURL, err := url.Parse(appConfig.IRCProxy)
		if err != nil {
			cancel()

			return "", err
		}

		dialer, err := proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
		if err != nil {
			cancel()

			return "", err
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
	if err != nil {

		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func ChatGPTRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	gptMemory *[]openai.ChatCompletionMessage,
	prompt string,
) string {
	resp, err := DoChatGPTRequest(appConfig, gptMemory, prompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: resp,
	})

	if len(*gptMemory) > appConfig.MemoryLimit {
		*gptMemory = []openai.ChatCompletionMessage{}
	}

	var writer bytes.Buffer

	err = quick.Highlight(
		&writer,
		resp,
		"markdown",
		appConfig.ChromaFormatter,
		appConfig.ChromaStyle)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	return writer.String()
}

func ChatGPTHandler(
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

		result := ChatGPTRequestProcessor(appConfig, client, event, gptMemory, prompt)
		if result != "" {
			sendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
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
		OllamaHandler(irc, &appConfig, &OllamaMemory)
	case "gemini":
		GeminiHandler(irc, &appConfig, &GeminiMemory)
	case "chatgpt":
		ChatGPTHandler(irc, &appConfig, &GPTMemory)
	}

	go LoadAllPlugins(&appConfig, irc)

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
