package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"index/suffixarray"
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
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
	"github.com/cenkalti/backoff/v5"
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

	if config.Temperature == 0 {
		config.Temperature = 0.5
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

	if config.OllamaMirostatEta == 0 {
		config.OllamaMirostatEta = 0.1
	}

	if config.OllamaMirostatTau == 0 {
		config.OllamaMirostatTau = 5.0
	}

	if config.OllamaNumCtx == 0 {
		config.OllamaNumCtx = 4096
	}

	if config.OllamaRepeatLastN == 0 {
		config.OllamaRepeatLastN = 64
	}

	if config.OllamaRepeatPenalty == 0 {
		config.OllamaRepeatPenalty = 1.1
	}

	if config.OllamaSeed == 0 {
		config.OllamaSeed = 42
	}

	if config.OllamaNumPredict == 0 {
		config.OllamaNumPredict = -1
	}

	if config.TopK == 0 {
		config.TopK = 40
	}

	if config.TopP == 0.0 {
		config.TopP = 0.9
	}

	if config.OllamaMinP == 0 {
		config.OllamaMinP = 0.05
	}

	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
}

func getTableFromChanName(channel, ircdName string) string {
	tableName := ircdName + "_" + channel
	tableName = strings.ReplaceAll(tableName, "#", "")
	tableName = strings.ReplaceAll(tableName, "-", "_")
	tableName = strings.TrimSpace(tableName)

	return tableName
}

func stripColorCodes(input string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	input = re.ReplaceAllString(input, "")
	re = regexp.MustCompile(`\x03(?:\d{1,2}(?:,\d{1,2})?)?`)
	input = re.ReplaceAllString(input, "")

	return input
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
	helpString += "load - loads a lua script\n"
	helpString += "unload - unloads a lua script\n"
	helpString += "remind - reminds you in a given amount of seconds\n"
	helpString += "roll - rolls a dice. the number is between 1 and 6. One arg sets the upper limit. Two args sets the lower and upper limit in that order\n"

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
	case reflect.Int32:
		intValue, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return errWrongDataForField
		}

		fieldValue.SetInt(int64(intValue))
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
		LogError(err)

		return
	}

	if customCommand.Limit != 0 {
		logs = logs[:customCommand.Limit]
	}

	log.Println(logs)

	if err != nil {
		LogError(err)

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

		for _, customContext := range customCommand.Context {
			gptMemory = append(gptMemory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: customContext,
			})
		}

		var bigPrompt string
		for _, log := range logs {
			bigPrompt += log.Log + "\n"
		}

		result := ChatGPTRequestProcessor(appConfig, client, event, &gptMemory, customCommand.Prompt, customCommand.SystemPrompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
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

		for _, customContext := range customCommand.Context {
			geminiMemory = append(geminiMemory, &genai.Content{
				Parts: []genai.Part{
					genai.Text(customContext),
				},
				Role: "model",
			})
		}

		result := GeminiRequestProcessor(appConfig, client, event, &geminiMemory, customCommand.Prompt, customCommand.SystemPrompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	case "ollama":
		var ollamaMemory []MemoryElement

		for _, log := range logs {
			ollamaMemory = append(ollamaMemory, MemoryElement{
				Role:    "user",
				Content: log.Log,
			})
		}

		for _, customContext := range customCommand.Context {
			ollamaMemory = append(ollamaMemory, MemoryElement{
				Role:    "assistant",
				Content: customContext,
			})
		}

		result := OllamaRequestProcessor(appConfig, client, event, &ollamaMemory, customCommand.Prompt, customCommand.SystemPrompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	case "openrouter":
		var memory []MemoryElement

		for _, log := range logs {
			memory = append(memory, MemoryElement{
				Role:    "user",
				Content: log.Log,
			})
		}

		for _, customContext := range customCommand.Context {
			memory = append(memory, MemoryElement{
				Role:    "user",
				Content: customContext,
			})
		}

		result := ORRequestProcessor(appConfig, client, event, &memory, customCommand.Prompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
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
		SendToIRC(client, event, getHelpString(), "noop")
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

			fieldValueString, ok := fieldValue.(string)
			if !ok {
				continue
			}

			client.Cmd.Reply(event, fmt.Sprintf("%s: %v", field.Name, fieldValueString))
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
		if len(args) == 3 {
			IrcJoin(client, []string{args[1], args[2]})
		} else {
			client.Cmd.Join(args[1])
		}
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

		for key, value := range appConfig.LuaCommands {
			if value.Path == args[1] {
				appConfig.deleteLuaCommand(key)

				break
			}
		}

		appConfig.deleteLstate(args[1])
	case "remind":
		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		seconds, err := strconv.Atoi(args[1])
		if err != nil {
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		client.Cmd.Reply(event, "Ok, I'll remind you in "+args[1]+" seconds.")
		time.Sleep(time.Duration(seconds) * time.Second)

		client.Cmd.ReplyTo(event, " Ping!")
	case "forget":
		client.Cmd.Reply(event, "I no longer even know whether you're supposed to wear or drink a camel.'")
	case "whois":
		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		ianaResponse := IANAWhoisGet(args[1], appConfig)
		client.Cmd.Reply(event, ianaResponse)
	case "roll":
		lowerLimit := 1
		upperLimit := 6

		if len(args) == 1 {
		} else if len(args) == 2 { //nolint: mnd,gomnd
			argOne, err := strconv.Atoi(args[1])
			if err != nil {
				client.Cmd.Reply(event, errNotEnoughArgs.Error())

				break
			}

			upperLimit = argOne
		} else if len(args) == 3 { //nolint: mnd,gomnd
			argOne, err := strconv.Atoi(args[1])
			if err != nil {
				client.Cmd.Reply(event, errNotEnoughArgs.Error())

				break
			}

			lowerLimit = argOne

			argTwo, err := strconv.Atoi(args[2])
			if err != nil {
				client.Cmd.Reply(event, errNotEnoughArgs.Error())

				break
			}

			upperLimit = argTwo
		} else {
			client.Cmd.Reply(event, errors.New("too many args").Error())

			break
		}

		randomNumber := lowerLimit + rand.Intn(upperLimit-lowerLimit+1)

		client.Cmd.ReplyTo(event, fmt.Sprint(randomNumber))
	default:
		_, ok := appConfig.LuaCommands[args[0]]
		if !ok {
			client.Cmd.Reply(event, errUnknCmd.Error())

			break
		}

		luaArgs := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		luaArgs = strings.TrimSpace(luaArgs)
		luaArgs = strings.TrimPrefix(luaArgs, "/")
		luaArgs = strings.TrimPrefix(luaArgs, args[0])
		luaArgs = strings.TrimSpace(luaArgs)

		result := RunLuaFunc(args[0], luaArgs, client, appConfig)
		client.Cmd.Reply(event, result)
	}
}

func DoOllamaRequest(
	appConfig *TomlConfig,
	ollamaMemory *[]MemoryElement,
	prompt, systemPrompt string,
) (string, error) {
	var jsonPayload []byte

	var err error

	memoryElement := MemoryElement{
		Role:    "user",
		Content: prompt,
	}

	if len(*ollamaMemory) > appConfig.MemoryLimit {
		*ollamaMemory = []MemoryElement{}

		for _, context := range appConfig.Context {
			*ollamaMemory = append(*ollamaMemory, MemoryElement{
				Role:    "assistant",
				Content: context,
			})
		}
	}

	*ollamaMemory = append(*ollamaMemory, memoryElement)

	ollamaRequest := OllamaChatRequest{
		Model:     appConfig.Model,
		KeepAlive: time.Duration(appConfig.KeepAlive),
		Stream:    false,
		Messages:  *ollamaMemory,
		System:    systemPrompt,
		Options: OllamaRequestOptions{
			Mirostat:      appConfig.OllamaMirostat,
			MirostatEta:   appConfig.OllamaMirostatEta,
			MirostatTau:   appConfig.OllamaMirostatTau,
			NumCtx:        appConfig.OllamaNumCtx,
			RepeatLastN:   appConfig.OllamaRepeatLastN,
			RepeatPenalty: appConfig.OllamaRepeatPenalty,
			Temperature:   appConfig.Temperature,
			Seed:          appConfig.OllamaSeed,
			NumPredict:    appConfig.OllamaNumPredict,
			TopK:          appConfig.TopK,
			TopP:          appConfig.TopP,
			MinP:          appConfig.OllamaMinP,
		},
	}

	jsonPayload, err = json.Marshal(ollamaRequest)
	if err != nil {
		return "", err
	}

	log.Printf("json payload: %s", string(jsonPayload))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	request, err := http.NewRequest(http.MethodPost, appConfig.Endpoint, bytes.NewBuffer(jsonPayload))
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

	defer response.Body.Close()

	var ollamaChatResponse OllamaChatMessagesResponse

	err = json.NewDecoder(response.Body).Decode(&ollamaChatResponse)
	if err != nil {
		return "", err
	}

	log.Println("ollama chat response: ", ollamaChatResponse)

	return ollamaChatResponse.Messages.Content, nil
}

func OllamaRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	ollamaMemory *[]MemoryElement,
	prompt, systemPrompt string,
) string {
	response, err := DoOllamaRequest(appConfig, ollamaMemory, prompt, systemPrompt)
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

		result := OllamaRequestProcessor(appConfig, client, event, ollamaMemory, prompt, appConfig.SystemPrompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}

func (t *ProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if t.ProxyURL != "" {
		proxyURL, err := url.Parse(t.ProxyURL)
		if err != nil {
			return nil, err
		}

		transport.Proxy = http.ProxyURL(proxyURL)
	}

	newReq := req.Clone(req.Context())
	vals := newReq.URL.Query()
	vals.Set("key", t.APIKey)
	newReq.URL.RawQuery = vals.Encode()

	resp, err := transport.RoundTrip(newReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func DoGeminiRequest(
	appConfig *TomlConfig,
	geminiMemory *[]*genai.Content,
	prompt, systemPrompt string,
) (string, error) {
	httpProxyClient := &http.Client{Transport: &ProxyRoundTripper{
		APIKey:   appConfig.Apikey,
		ProxyURL: appConfig.LLMProxy,
	}}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	clientGemini, err := genai.NewClient(ctx, option.WithHTTPClient(httpProxyClient))
	if err != nil {
		return "", fmt.Errorf("Could not create a genai client.", err)
	}
	defer clientGemini.Close()

	model := clientGemini.GenerativeModel(appConfig.Model)
	model.SetTemperature(float32(appConfig.Temperature))
	model.SetTopK(appConfig.TopK)
	model.SetTopP(appConfig.TopP)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(systemPrompt),
		},
	}
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
	}

	cs := model.StartChat()

	cs.History = *geminiMemory

	resp, err := cs.SendMessage(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("Gemini: Could not send message", err)
	}

	return returnGeminiResponse(resp), nil
}

func GeminiRequestProcessor(
	appConfig *TomlConfig,
	client *girc.Client,
	event girc.Event,
	geminiMemory *[]*genai.Content,
	prompt, systemPrompt string,
) string {
	geminiResponse, err := DoGeminiRequest(appConfig, geminiMemory, prompt, systemPrompt)
	if err != nil {
		client.Cmd.ReplyTo(event, "error: "+err.Error())

		return ""
	}

	log.Println(geminiResponse)

	if len(*geminiMemory) > appConfig.MemoryLimit {
		*geminiMemory = []*genai.Content{}

		for _, context := range appConfig.Context {
			*geminiMemory = append(*geminiMemory, &genai.Content{
				Parts: []genai.Part{
					genai.Text(context),
				},
				Role: "model",
			})
		}
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

		result := GeminiRequestProcessor(appConfig, client, event, geminiMemory, prompt, appConfig.SystemPrompt)

		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}

func DoChatGPTRequest(
	appConfig *TomlConfig,
	gptMemory *[]openai.ChatCompletionMessage,
	prompt, systemPrompt string,
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

	if appConfig.Endpoint != "" {
		config.BaseURL = appConfig.Endpoint
		log.Print(config.BaseURL)
	}

	gptClient := openai.NewClientWithConfig(config)

	*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

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
	prompt, systemPrompt string,
) string {
	resp, err := DoChatGPTRequest(appConfig, gptMemory, prompt, systemPrompt)
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

		for _, context := range appConfig.Context {
			*gptMemory = append(*gptMemory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: context,
			})
		}
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

		result := ChatGPTRequestProcessor(appConfig, client, event, gptMemory, prompt, appConfig.SystemPrompt)
		if result != "" {
			SendToIRC(client, event, result, appConfig.ChromaFormatter)
		}
	})
}

func connectToDB(appConfig *TomlConfig, ctx *context.Context, irc *girc.Client) {
	var pool *pgxpool.Pool

	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s/%s",
		appConfig.DatabaseUser,
		appConfig.DatabasePassword,
		appConfig.DatabaseAddress,
		appConfig.DatabaseName)

	log.Println("dbURL:", dbURL)

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		LogError(err)

		return
	}

	dbConnect := func() (*pgxpool.Pool, error) {
		return pgxpool.NewWithConfig(*ctx, poolConfig)
	}

	pool, err = backoff.Retry(*ctx, dbConnect, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		LogError(err)
	}

	log.Printf("%s connected to database", appConfig.IRCDName)

	for _, channel := range appConfig.ScrapeChannels {
		tableName := getTableFromChanName(channel[0], appConfig.IRCDName)
		query := fmt.Sprintf(
			`create table if not exists %s (
						id serial primary key,
						channel text not null,
						log text not null,
						nick text not null,
						dateadded timestamp default current_timestamp
					)`, tableName)

		_, err := pool.Exec(*ctx, query)
		if err != nil {
			LogError(err)

			continue
		}
	}

	appConfig.pool = pool
}

func scrapeChannel(irc *girc.Client, appConfig *TomlConfig) {
	log.Print("spawning scraper")

	irc.Handlers.AddBg(girc.PRIVMSG, func(_ *girc.Client, event girc.Event) {
		if appConfig.pool == nil {
			log.Println("no db connection. cant write scrapes to db.")

			return
		}

		tableName := getTableFromChanName(event.Params[0], appConfig.IRCDName)
		query := fmt.Sprintf(
			"insert into %s (channel,log,nick) values ('%s','%s','%s')",
			tableName,
			sanitizeLog(event.Params[0]),
			sanitizeLog(stripColorCodes(event.Last())),
			event.Source.Name,
		)

		log.Println(query)

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
		defer cancel()

		_, err := appConfig.pool.Exec(ctx, query)
		if err != nil {
			LogError(err)
		}
	})
}

func populateWatchListWords(appConfig *TomlConfig) {
	for watchlistName, watchlist := range appConfig.WatchLists {
		for _, filepath := range watchlist.WatchFiles {
			filebytes, err := os.ReadFile(filepath)
			if err != nil {
				LogError(err)

				continue
			}

			filestring := string(filebytes)

			words := strings.Split(filestring, "\n")

			watchlist.Words = append(watchlist.Words, words...)
			appConfig.WatchLists[watchlistName] = watchlist
		}
	}
}

func WatchListHandler(irc *girc.Client, appConfig TomlConfig) {
	irc.Handlers.AddBg(girc.ALL_EVENTS, func(_ *girc.Client, event girc.Event) {
		var isRightEventType bool

		sarray := suffixarray.New([]byte(event.Last()))

		if len(event.Params) == 0 {
			return
		}

		for watchname, watchlist := range appConfig.WatchLists {
			for _, channel := range watchlist.WatchList {
				isRightEventType = false

				if channel[0] == event.Params[0] {

					for _, eventType := range watchlist.EventTypes {
						if eventType == event.Command {
							isRightEventType = true

							break
						}
					}

					if !isRightEventType {
						continue
					}

					for _, word := range watchlist.Words {
						indexes := sarray.Lookup([]byte(" "+word+" "), 1)
						if len(indexes) > 0 {
							nextWhitespaceIndex := strings.Index(event.Last()[indexes[0]+1:], " ")

							rewrittenMessage :=
								event.Last()[:indexes[0]+1] +
									fmt.Sprintf("\x1b[48;5;%dm", watchlist.BGColor) +
									fmt.Sprintf("\x1b[38;5;%dm", watchlist.FGColor) +
									event.Last()[indexes[0]+1:indexes[0]+1+nextWhitespaceIndex] +
									"\x1b[0m" + event.Last()[indexes[0]+1+nextWhitespaceIndex:]

							irc.Cmd.Message(
								watchlist.AlertChannel[0],
								fmt.Sprintf("%s: %s", watchname, rewrittenMessage))

							log.Printf("matched from watchlist -- %s: %s", watchname, event.Last())

							break
						}
					}
				}
			}
		}
	})
}

func runIRC(appConfig TomlConfig) {
	var OllamaMemory []MemoryElement

	var GeminiMemory []*genai.Content

	var GPTMemory []openai.ChatCompletionMessage

	var ORMemory []MemoryElement

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

	irc.Handlers.AddBg(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) {
		for _, channel := range appConfig.IrcChannels {
			IrcJoin(irc, channel)
		}
	})

	switch appConfig.Provider {
	case "ollama":
		for _, context := range appConfig.Context {
			OllamaMemory = append(OllamaMemory, MemoryElement{
				Role:    "assistant",
				Content: context,
			})
		}

		OllamaHandler(irc, &appConfig, &OllamaMemory)
	case "gemini":
		for _, context := range appConfig.Context {
			GeminiMemory = append(GeminiMemory, &genai.Content{
				Parts: []genai.Part{
					genai.Text(context),
				},
				Role: "model",
			})
		}

		GeminiHandler(irc, &appConfig, &GeminiMemory)
	case "chatgpt":
		for _, context := range appConfig.Context {
			GPTMemory = append(GPTMemory, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: context,
			})
		}

		ChatGPTHandler(irc, &appConfig, &GPTMemory)
	case "openrouter":
		for _, context := range appConfig.Context {
			ORMemory = append(ORMemory, MemoryElement{
				Role:    "user",
				Content: context,
			})
		}

		ORHandler(irc, &appConfig, &ORMemory)
	}

	go LoadAllPlugins(&appConfig, irc)

	if appConfig.DatabaseAddress != "" {
		context, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
		defer cancel()

		go connectToDB(&appConfig, &context, irc)
	}

	if len(appConfig.ScrapeChannels) > 0 {
		irc.Handlers.AddBg(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) {
			for _, channel := range appConfig.ScrapeChannels {
				IrcJoin(irc, channel)
			}
		})

		go scrapeChannel(irc, &appConfig)
	}

	if len(appConfig.WatchLists) > 0 {
		irc.Handlers.AddBg(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) {
			for _, watchlist := range appConfig.WatchLists {
				log.Print("joining ", watchlist.AlertChannel)
				IrcJoin(irc, watchlist.AlertChannel)

				for _, channel := range watchlist.WatchList {
					IrcJoin(irc, channel)
				}
			}
		})

		populateWatchListWords(&appConfig)

		go WatchListHandler(irc, appConfig)
	}

	if len(appConfig.Rss) > 0 {
		irc.Handlers.AddBg(girc.CONNECTED, func(client *girc.Client, _ girc.Event) {
			for _, rss := range appConfig.Rss {
				log.Print("RSS: joining ", rss.Channel)
				IrcJoin(irc, rss.Channel)
			}
		})

		go runRSS(&appConfig, irc)
	}

	var dialer proxy.Dialer

	if appConfig.IRCProxy != "" {
		proxyURL, err := url.Parse(appConfig.IRCProxy)
		if err != nil {
			LogErrorFatal(err)
		}

		dialer, err = proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
		if err != nil {
			LogErrorFatal(err)
		}
	}

	connectToIRC := func() (string, error) {
		return "", irc.DialerConnect(dialer)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.MillaReconnectDelay)*time.Second)
	defer cancel()

	_, err := backoff.Retry(ctx, connectToIRC, backoff.WithBackOff(backoff.NewExponentialBackOff()))
	if err != nil {
		LogError(err)
	} else {
		return
	}
}

func goroutines() interface{} {
	return runtime.NumGoroutine()
}

func main() {
	expvar.Publish("Goroutines", expvar.Func(goroutines))

	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)

	configPath := flag.String("config", "./config.toml", "path to the config file")

	flag.Parse()

	data, err := os.ReadFile(*configPath)
	if err != nil {
		LogErrorFatal(err)
	}

	var config AppConfig

	_, err = toml.Decode(string(data), &config)
	if err != nil {
		LogErrorFatal(err)
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

	go func() {
		err := http.ListenAndServe(":6060", nil)
		log.Println(err)
	}()

	<-quitChannel
}
