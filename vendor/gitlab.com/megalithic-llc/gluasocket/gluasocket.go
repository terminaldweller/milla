package gluasocket

import (
	"github.com/yuin/gopher-lua"
	luaScripts "gitlab.com/megalithic-llc/gluasocket/lua"
	"gitlab.com/megalithic-llc/gluasocket/mimecore"
	"gitlab.com/megalithic-llc/gluasocket/socketcore"
	"gitlab.com/megalithic-llc/gluasocket/socketexcept"
	"gitlab.com/megalithic-llc/gluasocket/sockethttp"
)

func Preload(L *lua.LState) {
	L.PreloadModule("ltn12", luaScripts.Ltn12Loader)
	L.PreloadModule("mime.core", mimecore.Loader)
	L.PreloadModule("mime", luaScripts.MimeLoader)
	L.PreloadModule("socket", luaScripts.SocketLoader)
	L.PreloadModule("socket.core", socketcore.Loader)
	L.PreloadModule("socket.except", socketexcept.Loader)
	L.PreloadModule("socket.ftp", luaScripts.FtpLoader)
	L.PreloadModule("socket.headers", luaScripts.HeadersLoader)
	L.PreloadModule("socket.http", sockethttp.Loader)
	L.PreloadModule("socket.smtp", luaScripts.SmtpLoader)
	L.PreloadModule("socket.tp", luaScripts.TpLoader)
	L.PreloadModule("socket.url", luaScripts.UrlLoader)
}
