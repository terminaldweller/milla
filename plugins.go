package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/ailncode/gluaxmlpath"
	"github.com/cjoudrey/gluahttp"
	"github.com/jackc/pgx/v5"
	"github.com/kohkimakimoto/gluayaml"
	gopherjson "github.com/layeh/gopher-json"
	"github.com/lrstanley/girc"
	openai "github.com/sashabaranov/go-openai"
	"github.com/yuin/gluare"
	lua "github.com/yuin/gopher-lua"
	"gitlab.com/megalithic-llc/gluasocket"
	"google.golang.org/genai"
)

func registerStructAsLuaMetaTable[T any](
	luaState *lua.LState,
	luaLTable *lua.LTable,
	checkStruct func(luaState *lua.LState) *T,
	structType T,
	metaTableName string,
) {
	metaTable := luaState.NewTypeMetatable(metaTableName)

	luaState.SetField(luaLTable, metaTableName, metaTable)

	luaState.SetField(
		metaTable,
		"new",
		luaState.NewFunction(
			newStructFunctionFactory(structType, metaTableName),
		),
	)

	var dummyType T
	tableMethods := luaTableGenFactory(reflect.TypeOf(dummyType), checkStruct)

	luaState.SetField(
		metaTable,
		"__index",
		luaState.SetFuncs(
			luaState.NewTable(),
			tableMethods,
		),
	)
}

func newStructFunctionFactory[T any](structType T, metaTableName string) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		structInstance := &structType
		ud := luaState.NewUserData()
		ud.Value = structInstance
		luaState.SetMetatable(ud, luaState.GetTypeMetatable(metaTableName))
		luaState.Push(ud)

		return 1
	}
}

func checkStruct[T any](luaState *lua.LState) *T {
	userData := luaState.CheckUserData(1)

	if v, ok := userData.Value.(*T); ok {
		return v
	}

	luaState.ArgError(1, "got wrong struct")

	return nil
}

func getterSetterFactory[T any](
	fieldName string,
	fieldType reflect.Type,
	checkStruct func(luaState *lua.LState) *T,
) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		genericStruct := checkStruct(luaState)

		structValue := reflect.ValueOf(genericStruct).Elem()

		fieldValue := structValue.FieldByName(fieldName)

		if luaState.GetTop() == 2 { //nolint: mnd,gomnd
			switch fieldType.Kind() {
			case reflect.String:
				fieldValue.SetString(luaState.CheckString(2)) //nolint: mnd,gomnd
			case reflect.Float64:
				fieldValue.SetFloat(float64(luaState.CheckNumber(2))) //nolint: mnd,gomnd
			case reflect.Float32:
				fieldValue.SetFloat(float64(luaState.CheckNumber(2))) //nolint: mnd,gomnd
			case reflect.Int8:
				fieldValue.SetInt(int64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Int16:
				fieldValue.SetInt(int64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Int:
				fieldValue.SetInt(int64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Int32:
				fieldValue.SetInt(int64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Int64:
				fieldValue.SetInt(int64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Bool:
				fieldValue.SetBool(luaState.CheckBool(2)) //nolint: mnd,gomnd
			case reflect.Uint:
				fieldValue.SetUint(uint64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Uint8:
				fieldValue.SetUint(uint64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Uint16:
				fieldValue.SetUint(uint64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Uint32:
				fieldValue.SetUint(uint64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Uint64:
				fieldValue.SetUint(uint64(luaState.CheckInt(2))) //nolint: mnd,gomnd
			case reflect.Func:
			case reflect.Ptr:
			case reflect.Struct:
			case reflect.Slice:
			case reflect.Array:
			case reflect.Map:
			default:
				log.Print("unsupported type")
			}

			return 0
		}

		switch fieldType.Kind() {
		case reflect.String:
			luaState.Push(lua.LString(fieldValue.Interface().(string)))
		case reflect.Float64:
			luaState.Push(lua.LNumber(fieldValue.Interface().(float64)))
		case reflect.Float32:
			luaState.Push(lua.LNumber(fieldValue.Float()))
		case reflect.Int8:
			luaState.Push(lua.LNumber(fieldValue.Int()))
		case reflect.Int16:
			luaState.Push(lua.LNumber(fieldValue.Int()))
		case reflect.Int:
			luaState.Push(lua.LNumber(fieldValue.Int()))
		case reflect.Int32:
			luaState.Push(lua.LNumber(fieldValue.Int()))
		case reflect.Int64:
			luaState.Push(lua.LNumber(fieldValue.Int()))
		case reflect.Bool:
			luaState.Push(lua.LBool(fieldValue.Bool()))
		case reflect.Uint:
			luaState.Push(lua.LNumber(fieldValue.Uint()))
		case reflect.Uint8:
			luaState.Push(lua.LNumber(fieldValue.Uint()))
		case reflect.Uint16:
			luaState.Push(lua.LNumber(fieldValue.Uint()))
		case reflect.Uint32:
			luaState.Push(lua.LNumber(fieldValue.Uint()))
		case reflect.Uint64:
			luaState.Push(lua.LNumber(fieldValue.Uint()))
		case reflect.Func:
		case reflect.Ptr:
		case reflect.Struct:
		case reflect.Slice:
		case reflect.Array:
		case reflect.Map:
		default:
			log.Print("unsupported type")
		}

		return 1
	}
}

func luaTableGenFactory[T any](
	structType reflect.Type,
	checkStructType func(luaState *lua.LState) *T) map[string]lua.LGFunction {
	tableMethods := make(map[string]lua.LGFunction)

	for _, field := range reflect.VisibleFields(structType) {
		tableMethods[field.Name] = getterSetterFactory(field.Name, field.Type, checkStructType)
	}

	return tableMethods
}

func sendMessageClosure(luaState *lua.LState, client *girc.Client) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		message := luaState.CheckString(1)
		target := luaState.CheckString(2) //nolint: mnd,gomnd

		client.Cmd.Message(target, message)

		return 0
	}
}

func replyToMessageClosure(luaStete *lua.LState, client *girc.Client, event girc.Event) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		message := luaState.CheckString(1)

		client.Cmd.Message(event.Source.Name, message)

		return 0
	}
}

func registerLuaCommand(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		path := luaState.CheckString(1)
		commandName := luaState.CheckString(2) //nolint: mnd,gomnd
		funcName := luaState.CheckString(3)    //nolint: mnd,gomnd

		_, ok := appConfig.LuaCommands[commandName]
		if ok {
			log.Print("command already registered: ", commandName)

			return 0
		}

		appConfig.insertLuaCommand(commandName, path, funcName)

		log.Print("registered command: ", commandName, path, funcName)

		return 0
	}
}

func registerTriggeredScript(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		path := luaState.CheckString(1)
		funcName := luaState.CheckString(3)       //nolint: mnd,gomnd
		eventTypesTable := luaState.CheckTable(2) //nolint: mnd,gomnd

		_, ok := appConfig.TriggeredScripts[path]
		if ok {
			log.Print("triggered script already registered: ", path)

			return 0
		}

		var eventTypes []string

		eventTypesTable.ForEach(func(_, value lua.LValue) {
			if value.Type() != lua.LTString {
				log.Print("event type for TriggeredScripts must be a string")
			} else {
				eventTypes = append(eventTypes, value.String())
			}
		})

		appConfig.insertTriggeredScript(path, funcName, eventTypes)

		return 0
	}
}

func ircJoinChannelClosure(luaState *lua.LState, client *girc.Client) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		channel := luaState.CheckString(1)
		password := luaState.CheckString(2) //nolint: mnd,gomnd

		if password != "" {
			client.Cmd.JoinKey(channel, password)
		} else {
			client.Cmd.Join(channel)
		}

		return 0
	}
}

func ircPartChannelClosure(luaState *lua.LState, client *girc.Client) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		channel := luaState.CheckString(1)

		client.Cmd.Part(channel)

		return 0
	}
}

func orRequestClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		prompt := luaState.CheckString(1)

		result, err := DoORRequest(appConfig, &[]MemoryElement{}, prompt)
		if err != nil {
			LogError(err)
		}

		luaState.Push(lua.LString(result))

		return 1
	}
}

func ollamaRequestClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		prompt := luaState.CheckString(1)
		systemPrompt := luaState.CheckString(2) //nolint: mnd,gomnd

		result, err := DoOllamaRequest(appConfig, &[]MemoryElement{}, prompt, systemPrompt)
		if err != nil {
			LogError(err)
		}

		luaState.Push(lua.LString(result))

		return 1
	}
}

func geminiRequestClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		prompt := luaState.CheckString(1)
		systemPrompt := luaState.CheckString(2) //nolint: mnd,gomnd

		result, err := DoGeminiRequest(appConfig, &[]*genai.Content{}, prompt, systemPrompt)
		if err != nil {
			LogError(err)
		}

		luaState.Push(lua.LString(result))

		return 1
	}
}

func chatGPTRequestClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		prompt := luaState.CheckString(1)
		systemPrompt := luaState.CheckString(2) //nolint: mnd,gomnd

		result, err := DoChatGPTRequest(appConfig, &[]openai.ChatCompletionMessage{}, prompt, systemPrompt)
		if err != nil {
			LogError(err)
		}

		luaState.Push(lua.LString(result))

		return 1
	}
}

func dbQueryClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		if appConfig.pool == nil {
			log.Println("Database connection is not available")

			return 0
		}

		query := luaState.CheckString(1)

		rows, err := appConfig.pool.Query(context.Background(), query)
		if err != nil {
			LogError(err)
		}
		defer rows.Close()

		logs, err := pgx.CollectRows(rows, pgx.RowToStructByName[LogModel])
		if err != nil {
			LogError(err)
		}

		table := luaState.CreateTable(0, len(logs))

		for index, log := range logs {
			luaState.SetTable(table, lua.LNumber(index), lua.LString(log.Log))
		}

		luaState.Push(table)

		return 1
	}
}

func urlEncode(luaState *lua.LState) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		URL := luaState.CheckString(1)
		escapedURL := url.QueryEscape(URL)
		luaState.Push(lua.LString(escapedURL))

		return 1
	}
}

func millaModuleLoaderClosure(luaState *lua.LState, client *girc.Client, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		exports := map[string]lua.LGFunction{
			"send_message":         lua.LGFunction(sendMessageClosure(luaState, client)),
			"join_channel":         lua.LGFunction(ircJoinChannelClosure(luaState, client)),
			"part_channel":         lua.LGFunction(ircPartChannelClosure(luaState, client)),
			"send_ollama_request":  lua.LGFunction(ollamaRequestClosure(luaState, appConfig)),
			"send_gemini_request":  lua.LGFunction(geminiRequestClosure(luaState, appConfig)),
			"send_chatgpt_request": lua.LGFunction(chatGPTRequestClosure(luaState, appConfig)),
			"send_or_request":      lua.LGFunction(orRequestClosure(luaState, appConfig)),
			"query_db":             lua.LGFunction(dbQueryClosure(luaState, appConfig)),
			"register_cmd":         lua.LGFunction(registerLuaCommand(luaState, appConfig)),
			"url_encode":           lua.LGFunction(urlEncode(luaState)),
		}
		millaModule := luaState.SetFuncs(luaState.NewTable(), exports)

		registerStructAsLuaMetaTable[TomlConfig](luaState, millaModule, checkStruct, TomlConfig{}, "toml_config")
		registerStructAsLuaMetaTable[CustomCommand](luaState, millaModule, checkStruct, CustomCommand{}, "custom_command")
		registerStructAsLuaMetaTable[LogModel](luaState, millaModule, checkStruct, LogModel{}, "log_model")
		registerStructAsLuaMetaTable[girc.Source](luaState, millaModule, checkStruct, girc.Source{}, "girc_source")
		// FIXME: throws error
		// registerStructAsLuaMetaTable[girc.Tags](luaState, millaModule, checkStruct, girc.Tags{}, "girc_tags")
		registerStructAsLuaMetaTable[girc.Event](luaState, millaModule, checkStruct, girc.Event{}, "girc_event")

		luaState.SetGlobal("milla", millaModule)

		luaState.Push(millaModule)

		return 1
	}
}

func millaModuleLoaderEventClosure(luaState *lua.LState, client *girc.Client, appConfig *TomlConfig, event girc.Event) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		exports := map[string]lua.LGFunction{
			"send_message":         lua.LGFunction(sendMessageClosure(luaState, client)),
			"reply_to":             lua.LGFunction(replyToMessageClosure(luaState, client, event)),
			"join_channel":         lua.LGFunction(ircJoinChannelClosure(luaState, client)),
			"part_channel":         lua.LGFunction(ircPartChannelClosure(luaState, client)),
			"send_ollama_request":  lua.LGFunction(ollamaRequestClosure(luaState, appConfig)),
			"send_gemini_request":  lua.LGFunction(geminiRequestClosure(luaState, appConfig)),
			"send_chatgpt_request": lua.LGFunction(chatGPTRequestClosure(luaState, appConfig)),
			"send_or_request":      lua.LGFunction(orRequestClosure(luaState, appConfig)),
			"query_db":             lua.LGFunction(dbQueryClosure(luaState, appConfig)),
			"register_cmd":         lua.LGFunction(registerLuaCommand(luaState, appConfig)),
			"url_encode":           lua.LGFunction(urlEncode(luaState)),
		}
		millaModule := luaState.SetFuncs(luaState.NewTable(), exports)

		registerStructAsLuaMetaTable[TomlConfig](luaState, millaModule, checkStruct, TomlConfig{}, "toml_config")
		registerStructAsLuaMetaTable[CustomCommand](luaState, millaModule, checkStruct, CustomCommand{}, "custom_command")
		registerStructAsLuaMetaTable[LogModel](luaState, millaModule, checkStruct, LogModel{}, "log_model")
		registerStructAsLuaMetaTable[girc.Source](luaState, millaModule, checkStruct, girc.Source{}, "girc_source")
		registerStructAsLuaMetaTable[girc.Event](luaState, millaModule, checkStruct, girc.Event{}, "girc_event")

		luaState.SetGlobal("milla", millaModule)

		luaState.Push(millaModule)

		return 1
	}
}

func RunScript(scriptPath string, client *girc.Client, appConfig *TomlConfig) {
	luaState := lua.NewState()
	defer luaState.Close()

	ctx, cancel := context.WithCancel(context.Background())

	luaState.SetContext(ctx)

	appConfig.insertLState(scriptPath, luaState, cancel)

	luaState.PreloadModule("milla", millaModuleLoaderClosure(luaState, client, appConfig))
	gluasocket.Preload(luaState)
	gluaxmlpath.Preload(luaState)
	luaState.PreloadModule("yaml", gluayaml.Loader)
	luaState.PreloadModule("re", gluare.Loader)
	luaState.PreloadModule("json", gopherjson.Loader)

	var proxyString string
	if os.Getenv("ALL_PROXY") != "" {
		proxyString = os.Getenv("ALL_PROXY")
	} else if os.Getenv("HTTPS_PROXY") != "" {
		proxyString = os.Getenv("HTTPS_PROXY")
	} else if os.Getenv("HTTP_PROXY") != "" {
		proxyString = os.Getenv("HTTP_PROXY")
	} else if os.Getenv("https_proxy") != "" {
		proxyString = os.Getenv("https_proxy")
	} else if os.Getenv("http_proxy") != "" {
		proxyString = os.Getenv("http_proxy")
	}

	proxyTransport := &http.Transport{}

	if proxyString != "" {
		proxyURL, err := url.Parse(proxyString)
		if err != nil {
			LogError(err)
		}

		proxyTransport.Proxy = http.ProxyURL(proxyURL)
	}

	luaState.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{Transport: proxyTransport}).Loader)

	log.Print("Running script: ", scriptPath)

	err := luaState.DoFile(scriptPath)
	if err != nil {
		LogError(err)
	}
}

func LoadAllPlugins(appConfig *TomlConfig, client *girc.Client) {
	for _, scriptPath := range appConfig.Plugins {
		log.Print("Loading plugin: ", scriptPath)

		go RunScript(scriptPath, client, appConfig)
	}
}

func LoadAllEventPlugins(appConfig *TomlConfig, client *girc.Client) {
	for _, triggeredScript := range appConfig.TriggeredScripts {
		log.Print("Loading event plugin: ", triggeredScript.Path)

		go RunScript(triggeredScript.Path, client, appConfig)
		registerTriggeredScripts(client, *appConfig)
	}
}

func RunLuaFunc(
	cmd, args string,
	client *girc.Client,
	appConfig *TomlConfig,
) string {
	luaState := lua.NewState()
	defer luaState.Close()

	ctx, cancel := context.WithCancel(context.Background())

	luaState.SetContext(ctx)

	scriptPath := appConfig.LuaCommands[cmd].Path

	appConfig.insertLState(scriptPath, luaState, cancel)

	luaState.PreloadModule("milla", millaModuleLoaderClosure(luaState, client, appConfig))
	gluasocket.Preload(luaState)
	gluaxmlpath.Preload(luaState)
	luaState.PreloadModule("yaml", gluayaml.Loader)
	luaState.PreloadModule("re", gluare.Loader)
	luaState.PreloadModule("json", gopherjson.Loader)

	var proxyString string
	if os.Getenv("ALL_PROXY") != "" {
		proxyString = os.Getenv("ALL_PROXY")
	} else if os.Getenv("HTTPS_PROXY") != "" {
		proxyString = os.Getenv("HTTPS_PROXY")
	} else if os.Getenv("HTTP_PROXY") != "" {
		proxyString = os.Getenv("HTTP_PROXY")
	} else if os.Getenv("https_proxy") != "" {
		proxyString = os.Getenv("https_proxy")
	} else if os.Getenv("http_proxy") != "" {
		proxyString = os.Getenv("http_proxy")
	}

	log.Print("set proxy env to:", proxyString)

	proxyTransport := &http.Transport{}

	if proxyString != "" {
		proxyURL, err := url.Parse(proxyString)
		if err != nil {
			LogError(err)
		}

		proxyTransport.Proxy = http.ProxyURL(proxyURL)
	}

	luaState.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{Transport: proxyTransport}).Loader)

	log.Print("Running lua command script: ", scriptPath)

	if err := luaState.DoFile(scriptPath); err != nil {
		LogError(err)

		return ""
	}

	funcLValue := lua.P{
		Fn:      luaState.GetGlobal(appConfig.LuaCommands[cmd].FuncName),
		NRet:    1,
		Protect: true,
	}

	log.Print(cmd)
	log.Print(args)

	if err := luaState.CallByParam(funcLValue, lua.LString(args)); err != nil {
		log.Print("failed running lua command ...")
		LogError(err)

		return ""
	}

	result := luaState.Get(-1)
	luaState.Pop(1)

	return result.String()
}

func RunTriggeredLuaFunc(
	funcName, scriptPath string,
	client *girc.Client,
	event girc.Event,
	appConfig *TomlConfig,
) string {
	luaState := lua.NewState()
	defer luaState.Close()

	ctx, cancel := context.WithCancel(context.Background())

	luaState.SetContext(ctx)

	appConfig.insertLState(scriptPath, luaState, cancel)

	luaState.PreloadModule("milla", millaModuleLoaderEventClosure(luaState, client, appConfig, event))
	gluasocket.Preload(luaState)
	gluaxmlpath.Preload(luaState)
	luaState.PreloadModule("yaml", gluayaml.Loader)
	luaState.PreloadModule("re", gluare.Loader)
	luaState.PreloadModule("json", gopherjson.Loader)

	var proxyString string
	if os.Getenv("ALL_PROXY") != "" {
		proxyString = os.Getenv("ALL_PROXY")
	} else if os.Getenv("HTTPS_PROXY") != "" {
		proxyString = os.Getenv("HTTPS_PROXY")
	} else if os.Getenv("HTTP_PROXY") != "" {
		proxyString = os.Getenv("HTTP_PROXY")
	} else if os.Getenv("https_proxy") != "" {
		proxyString = os.Getenv("https_proxy")
	} else if os.Getenv("http_proxy") != "" {
		proxyString = os.Getenv("http_proxy")
	}

	log.Print("set proxy env to:", proxyString)

	proxyTransport := &http.Transport{}

	if proxyString != "" {
		proxyURL, err := url.Parse(proxyString)
		if err != nil {
			LogError(err)
		}

		proxyTransport.Proxy = http.ProxyURL(proxyURL)
	}

	luaState.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{Transport: proxyTransport}).Loader)

	log.Print("Running lua command script: ", scriptPath)

	if err := luaState.DoFile(scriptPath); err != nil {
		LogError(err)

		return ""
	}

	funcLValue := lua.P{
		Fn:      luaState.GetGlobal(funcName),
		NRet:    1,
		Protect: true,
	}

	if err := luaState.CallByParam(funcLValue); err != nil {
		log.Print("failed running lua command ...")
		LogError(err)

		return ""
	}

	result := luaState.Get(-1)
	luaState.Pop(1)

	return result.String()
}

func registerTriggeredScripts(irc *girc.Client, appConfig TomlConfig) {
	for _, triggeredScript := range appConfig.TriggeredScripts {
		for _, triggerType := range triggeredScript.TriggerTypes {
			switch triggerType {
			case girc.PRIVMSG:
				irc.Handlers.AddBg(girc.PRIVMSG, func(_ *girc.Client, event girc.Event) {
					if !strings.HasPrefix(event.Last(), appConfig.IrcNick+": ") {
						return
					}

					if appConfig.AdminOnly && !isFromAdmin(appConfig.Admins, event) {
						return
					}

					RunTriggeredLuaFunc(triggeredScript.FuncName, triggeredScript.Path, irc, event, &appConfig)
				})
			default:
			}
		}
	}
}
