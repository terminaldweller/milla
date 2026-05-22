package lua

import (
	"embed"
	"github.com/yuin/gopher-lua"
)

var (
	//go:embed ftp.lua
	ftpDotLua embed.FS

	//go:embed headers.lua
	headersDotLua embed.FS

	//go:embed ltn12.lua
	ltn12DotLua embed.FS

	//go:embed mime.lua
	mimeDotLua embed.FS

	//go:embed socket.lua
	socketDotLua embed.FS

	//go:embed smtp.lua
	smtpDotLua embed.FS

	//go:embed tp.lua
	tpDotLua embed.FS

	//go:embed url.lua
	urlDotLua embed.FS
)

func loader(L *lua.LState, fs embed.FS, filename string) int {
	if script, err := fs.ReadFile(filename); err != nil {
		L.RaiseError("Error reading %s: %v", script, err)
		return 0
	} else if err := L.DoString(string(script)); err != nil {
		L.RaiseError("Error loading %s: %v", script, err)
		return 0
	}
	return 1
}

func FtpLoader(L *lua.LState) int {
	return loader(L, ftpDotLua, "ftp.lua")
}

func HeadersLoader(L *lua.LState) int {
	return loader(L, headersDotLua, "headers.lua")
}

func Ltn12Loader(L *lua.LState) int {
	return loader(L, ltn12DotLua, "ltn12.lua")
}

func MimeLoader(L *lua.LState) int {
	return loader(L, mimeDotLua, "mime.lua")
}

func SocketLoader(L *lua.LState) int {
	return loader(L, socketDotLua, "socket.lua")
}

func SmtpLoader(L *lua.LState) int {
	return loader(L, smtpDotLua, "smtp.lua")
}

func TpLoader(L *lua.LState) int {
	return loader(L, tpDotLua, "tp.lua")
}

func UrlLoader(L *lua.LState) int {
	return loader(L, urlDotLua, "url.lua")
}
