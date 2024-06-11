package main

import (
	"context"
	"log"
	"net/http"
	"reflect"

	"github.com/ailncode/gluaxmlpath"
	"github.com/cjoudrey/gluahttp"
	"github.com/google/generative-ai-go/genai"
	"github.com/jackc/pgx/v5"
	"github.com/kohkimakimoto/gluayaml"
	"github.com/lrstanley/girc"
	openai "github.com/sashabaranov/go-openai"
	"github.com/yuin/gluare"
	lua "github.com/yuin/gopher-lua"
	"gitlab.com/megalithic-llc/gluasocket"
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

func ircJoinChannelClosure(luaState *lua.LState, client *girc.Client) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		channel := luaState.CheckString(1)

		client.Cmd.Join(channel)

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

func ollamaRequestClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		prompt := luaState.CheckString(1)

		result, err := DoOllamaRequest(appConfig, &[]MemoryElement{}, prompt)
		if err != nil {
			log.Print(err)
		}

		luaState.Push(lua.LString(result))

		return 1
	}
}

func geminiRequestClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		prompt := luaState.CheckString(1)

		result, err := DoGeminiRequest(appConfig, &[]*genai.Content{}, prompt)
		if err != nil {
			log.Print(err)
		}

		luaState.Push(lua.LString(result))

		return 1
	}
}

func chatGPTRequestClosure(luaState *lua.LState, appConfig *TomlConfig) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		prompt := luaState.CheckString(1)

		result, err := DoChatGPTRequest(appConfig, &[]openai.ChatCompletionMessage{}, prompt)
		if err != nil {
			log.Print(err)
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
			log.Println(err.Error())
		}
		defer rows.Close()

		logs, err := pgx.CollectRows(rows, pgx.RowToStructByName[LogModel])
		if err != nil {
			log.Println(err.Error())
		}

		table := luaState.CreateTable(0, len(logs))

		for index, log := range logs {
			luaState.SetTable(table, lua.LNumber(index), lua.LString(log.Log))
		}

		luaState.Push(table)

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
			"query_db":             lua.LGFunction(dbQueryClosure(luaState, appConfig)),
		}
		millaModule := luaState.SetFuncs(luaState.NewTable(), exports)

		registerStructAsLuaMetaTable[TomlConfig](luaState, millaModule, checkStruct, TomlConfig{}, "toml_config")
		registerStructAsLuaMetaTable[CustomCommand](luaState, millaModule, checkStruct, CustomCommand{}, "custom_command")
		registerStructAsLuaMetaTable[LogModel](luaState, millaModule, checkStruct, LogModel{}, "log_model")

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
	luaState.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{}).Loader)
	luaState.PreloadModule("yaml", gluayaml.Loader)
	luaState.PreloadModule("re", gluare.Loader)

	log.Print("Running script: ", scriptPath)

	err := luaState.DoFile(scriptPath)
	if err != nil {
		log.Print(err)
	}
}

func LoadAllPlugins(appConfig *TomlConfig, client *girc.Client) {
	for _, scriptPath := range appConfig.Plugins {
		log.Print("Loading plugin: ", scriptPath)

		go RunScript(scriptPath, client, appConfig)
	}
}
