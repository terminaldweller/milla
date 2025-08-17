local milla = require("milla")
local os = require("os")
local json = require("json")

-- https://euvd.enisa.europa.eu/apidoc
function euvd(cli_args)
    local args = {}
    for i in string.gmatch(cli_args, "%S+") do table.insert(args, i) end

    os.setenv("http_proxy", "http://172.17.0.1:8120")

    local http = require("http")
    local url

    local url_latest =
        "https://euvdservices.enisa.europa.eu/api/lastvulnerabilities"
    local url_exploited =
        "https://euvdservices.enisa.europa.eu/api/exploitedvulnerabilities"
    local url_critical =
        "https://euvdservices.enisa.europa.eu/api/criticalvulnerabilities"

    if args[1] == "latest" then
        url = url_latest
    elseif args[1] == "exploited" then
        url = url_exploited
    elseif args[1] == "critical" then
        url = url_critical
    else
        return "Invalid command"
    end

    local response, err = http.request("GET", url, {timeout = "10s"})
    if err ~= nil then
        print(err)
        return err
    end
    print(response.body)

    local json_response, err = json.decode(response.body)
    if err ~= nil then
        print(err)
        return err
    end

    if response.status_code ~= 200 then
        return "Error: " .. response.status_code
    end

    local result = ""
    for k, v in ipairs(json_response) do
        result = result .. "id: " .. v["id"] .. "\n"
        result = result .. "description: " .. v["description"] .. "\n"
        result = result .. "datePublished: " .. v["datePublished"] .. "\n"
        result = result .. "dateUpdated: " .. v["dateUpdated"] .. "\n"
        result = result .. "baseScore: " .. v["baseScore"] .. "\n"
        result = result .. "baseScoreVersion: " .. v["baseScoreVersion"] .. "\n"
        result = result .. "baseScoreVector: " .. v["baseScoreVector"] .. "\n"
        result = result .. "references: " .. v["references"] .. "\n"
        result = result .. "aliases: " .. v["aliases"] .. "\n"
        result = result .. "assigner: " .. v["assigner"] .. "\n"
        result = result .. "epss: " .. v["epss"] .. "\n"
        result = result ..
                     "----------------------------------------------------------------" ..
                     "\n"
    end

    print(result)

    return result
end

milla.register_cmd("/plugins/euvd.lua", "euvd", "euvd")
