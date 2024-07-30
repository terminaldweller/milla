local milla = require("milla")
local yaml = require("yaml")
local http = require("http")
local xmlpath = require("xmlpath")

local function read_file(file)
    local f = assert(io.open(file, "rb"))
    local content = f:read("*all")
    f:close()
    return content
end

local function sleep(n) os.execute("sleep " .. tonumber(n)) end

local function get_config()
    local yaml_config = read_file("./plugins/rss.yaml")
    local config = yaml.parse(yaml_config)
    return config
end

local function get_rss_feed(config)
    local titles = {}
    local author_names = {}
    local uris = {}
    local rss_feed_list = {}

    for _, v in pairs(config.rssfeeds) do
        local response, err = http.request("GET", v.url)
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        local node, err = xmlpath.loadxml(response.body)
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end

        local path, err = xmlpath.compile("//entry/title")
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        local iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(titles, match:string())
        end

        path, err = xmlpath.compile("//entry/author/name")
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(author_names, match:string())
        end

        path, err = xmlpath.compile("//entry/author/uri")
        if err ~= nil then
            milla.send_message(err, "")
            goto continue
        end
        iterator = path:iter(node)
        for _, match in ipairs(iterator) do
            table.insert(uris, match:string())
        end
        ::continue::
    end

    for i = 1, #titles do
        table.insert(rss_feed_list,
                     author_names[i] .. ": " .. titles[i] .. " -- " .. uris[i])
    end

    return rss_feed_list
end

local function rss_feed()
    local config = get_config()
    while true do
        for _, v in pairs(get_rss_feed(config)) do
            milla.send_message(v, config.channel)
            sleep(config.period)
        end
    end
end

rss_feed()
