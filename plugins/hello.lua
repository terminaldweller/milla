local milla = require("milla")

function hello() milla.reply_to("hello") end

milla.register_cmd("/plugins/hello.lua", "hello", "hello")
