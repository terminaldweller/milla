package main

import (
	"context"
	"crypto/tls"
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
	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lrstanley/girc"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/net/proxy"
	"google.golang.org/genai"
)

var (
	errNotEnoughArgs     = errors.New("not enough arguments")
	errUnknCmd           = errors.New("unknown command")
	errUnknConfig        = errors.New("unknown config name")
	errCantSet           = errors.New("can't set field")
	errWrongDataForField = errors.New("wrong data type for field")
	errUnsupportedType   = errors.New("unsupported type")
)

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
			geminiMemory = append(geminiMemory, genai.NewContentFromText(log.Log, "user"))
		}

		for _, customContext := range customCommand.Context {
			geminiMemory = append(geminiMemory, genai.NewContentFromText(customContext, "model"))
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
	case "list":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		for key, value := range appConfig.LuaCommands {
			client.Cmd.Reply(event, fmt.Sprintf("%s: %s", key, value.Path))
		}
	case "remind":
		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Message(event.Source.Name, errNotEnoughArgs.Error())

			break
		}

		seconds, err := strconv.Atoi(args[1])
		if err != nil {
			client.Cmd.Reply(event, errNotEnoughArgs.Error())
			client.Cmd.Message(event.Source.Name, errNotEnoughArgs.Error())

			break
		}

		client.Cmd.Message(event.Source.Name, "Ok, I'll remind you in "+args[1]+" seconds.")
		time.Sleep(time.Duration(seconds) * time.Second)

		client.Cmd.Message(event.Source.Name, "Ping!")
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
	case "ua":
		if !isFromAdmin(appConfig.Admins, event) {
			break
		}

		if len(args) < 2 { //nolint: mnd,gomnd
			client.Cmd.Reply(event, errNotEnoughArgs.Error())

			break
		}

		var query string

		if len(args) >= 3 {
			query = strings.TrimPrefix(cmd, args[0])
		}

		log.Println("query:", query)
		response := UserAgentsGet(args[1], query, appConfig)

		// client.Cmd.Reply(event, response)
		SendToIRC(client, event, response, appConfig.ChromaFormatter)

	default:
		_, ok := appConfig.LuaCommands[args[0]]
		if ok {
			luaArgs := strings.TrimPrefix(event.Last(), appConfig.IrcNick+": ")
			luaArgs = strings.TrimSpace(luaArgs)
			luaArgs = strings.TrimPrefix(luaArgs, "/")
			luaArgs = strings.TrimPrefix(luaArgs, args[0])
			luaArgs = strings.TrimSpace(luaArgs)

			result := RunLuaFunc(args[0], luaArgs, client, appConfig)
			client.Cmd.Reply(event, result)

			break
		}

		_, ok = appConfig.Aliases[args[0]]
		if ok {
			dummyEvent := event
			dummyEvent.Params[len(dummyEvent.Params)-1] = appConfig.Aliases[args[0]].Alias

			runCommand(client, dummyEvent, appConfig)

			break
		}

		client.Cmd.Reply(event, errUnknCmd.Error())
	}
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

	expBackoff := backoff.WithBackOff(&backoff.ExponentialBackOff{
		InitialInterval:     time.Millisecond * time.Duration(appConfig.DbBackOffInitialInterval),
		RandomizationFactor: appConfig.DbBackOffRandomizationFactor,
		Multiplier:          appConfig.DbBackOffMultiplier,
		MaxInterval:         time.Second * time.Duration(appConfig.DbBackOffMaxInterval),
	})

	pool, err = backoff.Retry(*ctx, dbConnect, expBackoff)
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
			GeminiMemory = append(GeminiMemory, genai.NewContentFromText(context, "model"))
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

	go LoadAllEventPlugins(&appConfig, irc)

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

	for {
		expBackoff := backoff.WithBackOff(&backoff.ExponentialBackOff{
			InitialInterval:     time.Millisecond * time.Duration(appConfig.IrcBackOffInitialInterval),
			RandomizationFactor: appConfig.IrcBackOffRandomizationFactor,
			Multiplier:          appConfig.IrcBackOffMultiplier,
			MaxInterval:         time.Second * time.Duration(appConfig.IrcBackOffMaxInterval),
		})

		_, err := backoff.Retry(ctx, connectToIRC, expBackoff)
		if err != nil {
			log.Println(appConfig.Name)
			LogError(err)
		} else {
			return
		}
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
	prof := flag.Bool("prof", false, "enable prof server")

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
		AddSaneDefaults(&value)
		value.IRCDName = key
		config.Ircd[key] = value
	}

	for k, v := range config.Ircd {
		log.Println(k, v)
	}

	for _, v := range config.Ircd {
		if v.IrcServer != "" {
			go runIRC(v)
		} else {
			log.Println("Could not find server for irc connection in the config file. skipping. check your config for spelling errors maybe.")
		}
	}

	for k, v := range config.Ghost {
		go RunGhost(v, k)
	}

	if *prof {
		go func() {
			err := http.ListenAndServe(":6060", nil)
			log.Println(err)
		}()
	}

	<-quitChannel
}
