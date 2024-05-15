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
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/google/generative-ai-go/genai"
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
	dbConnection         *pgxpool.Pool //nolint:gochecknoglobals
)

type TomlConfig struct {
	IrcServer           string   `toml:"ircServer"`
	IrcNick             string   `toml:"ircNick"`
	IrcSaslUser         string   `toml:"ircSaslUser"`
	IrcSaslPass         string   `toml:"ircSaslPass"`
	OllamaEndpoint      string   `toml:"ollamaEndpoint"`
	Model               string   `toml:"model"`
	ChromaStyle         string   `toml:"chromaStyle"`
	ChromaFormatter     string   `toml:"chromaFormatter"`
	Provider            string   `toml:"provider"`
	Apikey              string   `toml:"apikey"`
	OllamaSystem        string   `toml:"ollamaSystem"`
	ClientCertPath      string   `toml:"clientCertPath"`
	ServerPass          string   `toml:"serverPass"`
	Bind                string   `toml:"bind"`
	Name                string   `toml:"name"`
	DatabaseAddress     string   `toml:"databaseAddress"`
	DatabasePassword    string   `toml:"databasePassword"`
	DatabaseUser        string   `toml:"databaseUser"`
	DatabaseName        string   `toml:"databaseName"`
	LLMProxy            string   `toml:"llmProxy"`
	IRCProxy            string   `toml:"ircProxy"`
	Temp                float64  `toml:"temp"`
	RequestTimeout      int      `toml:"requestTimeout"`
	MillaReconnectDelay int      `toml:"millaReconnectDelay"`
	IrcPort             int      `toml:"ircPort"`
	KeepAlive           int      `toml:"keepAlive"`
	MemoryLimit         int      `toml:"memoryLimit"`
	PingDelay           int      `toml:"pingDelay"`
	PingTimeout         int      `toml:"pingTimeout"`
	TopP                float32  `toml:"topP"`
	TopK                int32    `toml:"topK"`
	EnableSasl          bool     `toml:"enableSasl"`
	SkipTLSVerify       bool     `toml:"skipTLSVerify"`
	UseTLS              bool     `toml:"useTLS"`
	DisableSTSFallback  bool     `toml:"disableSTSFallback"`
	AllowFlood          bool     `toml:"allowFlood"`
	Debug               bool     `toml:"debug"`
	Out                 bool     `toml:"out"`
	Admins              []string `toml:"admins"`
	IrcChannels         []string `toml:"ircChannels"`
	ScrapeChannels      []string `toml:"scrapeChannels"`
}

func NewTomlConfig() *TomlConfig {
	return &TomlConfig{
		IrcNick:             "milla",
		IrcSaslUser:         "milla",
		ChromaStyle:         "rose-pine-moon",
		ChromaFormatter:     "noop",
		Provider:            "ollama",
		DatabaseAddress:     "postgres",
		DatabaseUser:        "milla",
		DatabaseName:        "milladb",
		Temp:                0.5,  //nolint:gomnd
		RequestTimeout:      10,   //nolint:gomnd
		MillaReconnectDelay: 30,   //nolint:gomnd
		IrcPort:             6697, //nolint:gomnd
		KeepAlive:           600,  //nolint:gomnd
		MemoryLimit:         20,   //nolint:gomnd
		PingDelay:           20,   //nolint:gomnd
		PingTimeout:         20,   //nolint:gomnd
		TopP:                0.9,  //nolint:gomnd
		EnableSasl:          false,
		SkipTLSVerify:       false,
		UseTLS:              true,
		AllowFlood:          false,
		DisableSTSFallback:  true,
		Debug:               false,
		Out:                 false,
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
	Model      string               `json:"model"`
	Stream     bool                 `json:"stream"`
	Keep_alive time.Duration        `json:"keep_alive"`
	Options    OllamaRequestOptions `json:"options"`
	Messages   []MemoryElement      `json:"messages"`
}

type MemoryElement struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
		client.Cmd.Reply(event, chunk)
	}
}

func getHelpString() string {
	helpString := "Commands:\n"
	helpString += "help - show this help message\n"
	helpString += "set - set a configuration value\n"
	helpString += "get - get a configuration value\n"
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

func runCommand(
	client *girc.Client,
	event girc.Event,
	appConfig *TomlConfig,
) {
	cmd := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimPrefix(cmd, "/")
	args := strings.Split(cmd, " ")

	messageFromAdmin := false

	for _, admin := range appConfig.Admins {
		if event.Source.Name == admin {
			messageFromAdmin = true

			break
		}
	}

	if !messageFromAdmin {
		return
	}

	switch args[0] {
	case "help":
		sendToIRC(client, event, getHelpString(), "noop")
	case "set":
		if len(args) < 3 { //nolint:gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		err := setFieldByName(reflect.ValueOf(appConfig).Elem(), args[1], args[2])
		if err != nil {
			client.Cmd.Reply(event, err.Error())
		}
	case "get":
		if len(args) < 2 { //nolint:gomnd
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
	default:
		client.Cmd.Reply(event, errUnknCmd.Error())
	}
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
		prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		log.Println(prompt)

		if string(prompt[0]) == "/" {
			runCommand(client, event, appConfig)

			return
		}

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
			Model:      appConfig.Model,
			Keep_alive: time.Duration(appConfig.KeepAlive),
			Stream:     false,
			Messages:   *ollamaMemory,
			Options: OllamaRequestOptions{
				Temperature: appConfig.Temp,
			},
		}
		jsonPayload, err = json.Marshal(ollamaRequest)
		log.Printf("json payload: %s", string(jsonPayload))
		if err != nil {
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
		defer cancel()

		request, err := http.NewRequest(http.MethodPost, appConfig.OllamaEndpoint, bytes.NewBuffer(jsonPayload))
		request = request.WithContext(ctx)
		if err != nil {
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
		}

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
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
		}
		defer response.Body.Close()

		var writer bytes.Buffer

		var ollamaChatResponse OllamaChatMessagesResponse
		err = json.NewDecoder(response.Body).Decode(&ollamaChatResponse)
		if err != nil {
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))
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
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
		}

		sendToIRC(client, event, writer.String(), appConfig.ChromaFormatter)
	})
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
		prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		log.Println(prompt)

		if string(prompt[0]) == "/" {
			runCommand(client, event, appConfig)

			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
		defer cancel()

		// api and http client dont work together
		// https://github.com/google/generative-ai-go/issues/80

		// httpClient := http.Client{}
		// allProxy := os.Getenv("ALL_PROXY")
		// if allProxy != "" {
		// 	proxyUrl, err := url.Parse(allProxy)
		// 	if err != nil {
		// 		client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

		// 		return
		// 	}
		// 	transport := &http.Transport{
		// 		Proxy: http.ProxyURL(proxyUrl),
		// 	}

		// 	httpClient.Transport = transport
		// }

		// clientGemini, err := genai.NewClient(ctx, option.WithAPIKey(appConfig.Apikey), option.WithHTTPClient(&httpClient))

		if appConfig.Apikey == "" {
			appConfig.Apikey = os.Getenv("MILLA_APIKEY")
		}

		clientGemini, err := genai.NewClient(ctx, option.WithAPIKey(appConfig.Apikey))
		if err != nil {
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
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
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
		}

		geminiResponse := returnGeminiResponse(resp)
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
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
		}

		sendToIRC(client, event, writer.String(), appConfig.ChromaFormatter)
	})
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
		prompt := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
		log.Println(prompt)

		if string(prompt[0]) == "/" {
			runCommand(client, event, appConfig)

			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
		defer cancel()

		var httpClient http.Client

		if appConfig.LLMProxy != "" {
			proxyURL, err := url.Parse(appConfig.IRCProxy)
			if err != nil {
				cancel()

				log.Fatal(err.Error())
			}

			dialer, err := proxy.FromURL(proxyURL, &net.Dialer{Timeout: time.Duration(appConfig.RequestTimeout) * time.Second})
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

		if appConfig.Apikey == "" {
			appConfig.Apikey = os.Getenv("MILLA_APIKEY")
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
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

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
			client.Cmd.ReplyTo(event, fmt.Sprintf("error: %s", err.Error()))

			return
		}

		sendToIRC(client, event, writer.String(), appConfig.ChromaFormatter)
	})
}

func connectToDB(appConfig TomlConfig, context *context.Context) {
	for {
		if appConfig.DatabaseUser == "" {
			appConfig.DatabaseUser = os.Getenv("MILLA_DB_USER")
		}

		if appConfig.DatabasePassword == "" {
			appConfig.DatabasePassword = os.Getenv("MILLA_DB_PASSWORD")
		}

		if appConfig.DatabaseAddress == "" {
			appConfig.DatabaseAddress = os.Getenv("MILLA_DB_ADDRESS")
		}

		if appConfig.DatabaseName == "" {
			appConfig.DatabaseName = os.Getenv("MILLA_DB_NAME")
		}

		dbURL := fmt.Sprintf(
			"postgres://%s:%s@%s/%s",
			appConfig.DatabaseUser,
			appConfig.DatabasePassword,
			appConfig.DatabaseAddress,
			appConfig.DatabaseName)

		conn, err := pgxpool.New(*context, dbURL)
		if err != nil {
			log.Println(err)
			time.Sleep(time.Duration(appConfig.MillaReconnectDelay) * time.Second)
		} else {
			for _, channel := range appConfig.ScrapeChannels {
				query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id SERIAL PRIMARY KEY,channel TEXT NOT NULL,log TEXT NOT NULL,nick TEXT NOT NULL,dateadded TIMESTAMP DEFAULT CURRENT_TIMESTAMP)",
					strings.ReplaceAll(channel, "#", ""))

				log.Println(query)

				_, err = conn.Query(*context, query)
				if err != nil {
					log.Println(err.Error())
					time.Sleep(time.Duration(appConfig.MillaReconnectDelay) * time.Second)
				}
			}

			dbConnection = conn
		}
	}
}

func scrapeChannel(irc *girc.Client) {
	irc.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, event girc.Event) {
		if dbConnection == nil {
			log.Println("missed logging message because currently not connected to db")

			return
		}
		query := fmt.Sprintf("INSERT INTO %s (channel,log,nick) VALUES ('%s','%s','%s')",
			strings.ReplaceAll(event.Params[0], "#", ""),
			event.Params[0],
			event.Last(),
			event.Source.Name,
		)
		log.Println(query)

		_, err := dbConnection.Query(
			context.Background(), query)
		if err != nil {
			log.Println(err.Error())
		}
	})
}

func runIRC(appConfig TomlConfig, ircChan chan *girc.Client, dbChan chan *pgxpool.Pool) {
	var OllamaMemory []MemoryElement

	var GeminiMemory []*genai.Content

	var GPTMemory []openai.ChatCompletionMessage

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
			InsecureSkipVerify: appConfig.SkipTLSVerify, // #nosec G402
			ServerName:         appConfig.IrcServer,
		},
	})

	if appConfig.Debug {
		irc.Config.Debug = os.Stdout
	}

	if appConfig.Out {
		irc.Config.Out = os.Stdout
	}

	if appConfig.ServerPass == "" {
		appConfig.ServerPass = os.Getenv("MILLA_SERVER_PASSWORD")
	}

	irc.Config.ServerPass = appConfig.ServerPass

	if appConfig.Bind != "" {
		irc.Config.Bind = appConfig.Bind
	}

	if appConfig.Name != "" {
		irc.Config.Name = appConfig.Name
	}

	saslUser := appConfig.IrcSaslUser

	var saslPass string

	if appConfig.IrcSaslPass == "" {
		saslPass = os.Getenv("MILLA_SASL_PASSWORD")
	} else {
		saslPass = appConfig.IrcSaslPass
	}

	if appConfig.EnableSasl && saslUser != "" && saslPass != "" {
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

	irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
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

	context, cancel := context.WithTimeout(context.Background(), time.Duration(appConfig.RequestTimeout)*time.Second)
	defer cancel()

	go connectToDB(appConfig, &context)

	if len(appConfig.ScrapeChannels) > 0 {
		irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
			for _, channel := range appConfig.ScrapeChannels {
				c.Cmd.Join(channel)
			}
		})

		go scrapeChannel(irc)
	}
	ircChan <- irc

	for {
		var dialer proxy.Dialer

		if appConfig.IRCProxy != "" {
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
	configPath := flag.String("config", "./config.toml", "path to the config file")

	flag.Parse()

	data, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	appConfig := NewTomlConfig()

	_, err = toml.Decode(string(data), &appConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(appConfig)

	ircChan := make(chan *girc.Client, 1)
	dbConn := make(chan *pgxpool.Pool, 1)

	runIRC(*appConfig, ircChan, dbConn)
}
