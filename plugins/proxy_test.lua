local milla = require("milla")
local os = require("os")
local json = require("json")

os.setenv("ALL_PROXY", "socks5://172.17.0.1:9057")

local http = require("http")

local response, err = http.request("GET", "https://icanhazip.com")
print(response.body)
