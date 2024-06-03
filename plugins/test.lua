local function sleep(n)
    local t0 = os.clock()
    while os.clock() - t0 <= n do end
end

local function printer()
    while true do
        send_message("Hello, World!", "#warroom")
        sleep(5)
    end
end

printer()
