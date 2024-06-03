package main

import (
	"log"
	"reflect"

	"github.com/lrstanley/girc"
	lua "github.com/yuin/gopher-lua"
)

func registerStructAsLuaMetaTable[T any](
	luaState *lua.LState,
	checkStruct func(luaState *lua.LState) *T,
	structType T,
	metaTableName string,
) {
	metaTable := luaState.NewTypeMetatable(metaTableName)

	luaState.SetGlobal(metaTableName, metaTable)

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

func RegisterCustomLuaTypes(luaState *lua.LState) {
	registerStructAsLuaMetaTable[TomlConfig](luaState, checkStruct, TomlConfig{}, "toml_config")
	registerStructAsLuaMetaTable[CustomCommand](luaState, checkStruct, CustomCommand{}, "custom_command")
	registerStructAsLuaMetaTable[LogModel](luaState, checkStruct, LogModel{}, "log_model")
}

func sendMessageClosure(luaState *lua.LState, client *girc.Client) func(*lua.LState) int {
	return func(luaState *lua.LState) int {
		message := luaState.CheckString(1)
		target := luaState.CheckString(2)

		client.Cmd.Message(target, message)

		return 0
	}
}

func RunScript(scriptPath string, client *girc.Client) {
	luaState := lua.NewState()
	defer luaState.Close()

	RegisterCustomLuaTypes(luaState)

	luaState.SetGlobal("send_message", luaState.NewFunction(sendMessageClosure(luaState, client)))

	log.Print("Running script: ", scriptPath)
	err := luaState.DoFile(scriptPath)

	if err != nil {
		log.Print(err)
	}
}

func LoadAllPlugins(appConfig *TomlConfig, client *girc.Client) {
	for _, scriptPath := range appConfig.Plugins {
		log.Print("Loading plugin: ", scriptPath)
		go RunScript(scriptPath, client)
	}
}
