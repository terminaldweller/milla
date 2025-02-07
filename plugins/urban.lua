local milla = require("milla")
local os = require("os")
local json = require("json")

os.setenv("ALL_PROXY", "socks5://172.17.0.1:9004")

local http = require("http")

function milla_urban(cli_args)
    local args = {}
    for i in string.gmatch(cli_args, "%S+") do table.insert(args, i) end

    for k, v in ipairs(args) do print(k, v) end

    local count = 1
    local term = ""

    local skip = false

    for i = 1, #args do
        if skip then
            skip = false
            goto continue
        end
        if args[i] == "-n" then
            count = tonumber(args[i + 1])
            skip = true
        else
            term = term .. args[i] .. " "
        end
        ::continue::
    end
    print("Term: " .. term)
    print("Count: " .. count)

    local user_agent =
        "Mozilla/5.0 (X11; U; Linux x86_64; pl-PL; rv:1.9.2.10) Gecko/20100922 Ubuntu/10.10 (maverick) Firefox/3.6.10"

    local escaped_term = milla.url_encode(term)
    local url = "http://api.urbandictionary.com/v0/define?term=" .. escaped_term
    local response, err = http.request("GET", url, {
        timeout = "10s",
        headers = {
            ["User-Agent"] = user_agent,
            ["Accept"] = "application/json",
            ["Host"] = "api.urbandictionary.com",
            ["Connection"] = "keep-alive",
            ["Cache-Control"] = "no-cache",
            ["DNT"] = 1,
            ["sec-ch-ua-platform"] = "Windows",
            ["Pragma"] = "no-cache"
        }
    })
    if err ~= nil then print(err) end
    print(response.body)

    local json_response, err = json.decode(response.body)
    if err ~= nil then print(err) end

    if response.status_code ~= 200 then
        return "Error: " .. response.status_code
    end

    local result = ""
    for _, v in ipairs(json_response["list"]) do
        for kk, vv in pairs(v) do print(kk, vv) end
        if count > 0 then
            result = result .. tostring(count) .. " - " .. v["definition"] ..
                         " ---- "
        end
        count = count - 1
    end

    return result
end

milla.register_cmd("/plugins/urban.lua", "urban", "milla_urban")
