package main

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestMetaTable(t *testing.T) {
	luaState := lua.NewState()
	defer luaState.Close()

	RegisterCustomLuaTypes(luaState)

	if err := luaState.DoString(`
		print("Testing MetaTable")
		print(toml_config)

		for index, data in ipairs(toml_config) do
			print(index, data)
			for k,v in pairs(data) do
				print("one")
				print(k,v)
			end
		end

		config = toml_config.new()
		print(config:IrcServer())
		config:IrcServer("irc.freenode.net")
		print(config:IrcServer())
	`); err != nil {
		t.Fatal(err)
	}
}
