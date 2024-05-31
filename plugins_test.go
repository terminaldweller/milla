package main

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func MetaTableTest(t *testing.T) {
	luaState := lua.NewState()
	defer luaState.Close()

	RegisterCustomLuaTypes(luaState)

	if err := luaState.DoString(`
		config = toml_config.new()
		print(config:IrcServer())
		config:TrcServer("irc.freenode.net")
		print(config:IrcServer())
	`); err != nil {
		t.Fatal(err)
	}
}
