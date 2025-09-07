package main

func AddSaneDefaults(config *TomlConfig) {
	if config.IrcNick == "" {
		config.IrcNick = "milla"
	}

	if config.ChromaStyle == "" {
		config.ChromaStyle = "rose-pine-moon"
	}

	if config.ChromaFormatter == "" {
		config.ChromaFormatter = "noop"
	}

	if config.DatabaseAddress == "" {
		config.DatabaseAddress = "postgres"
	}

	if config.DatabaseUser == "" {
		config.DatabaseUser = "milla"
	}

	if config.DatabaseName == "" {
		config.DatabaseName = "milladb"
	}

	if config.Temperature == 0 {
		config.Temperature = 0.5
	}

	if config.RequestTimeout == 0 {
		config.RequestTimeout = 10
	}

	if config.MillaReconnectDelay == 0 {
		config.MillaReconnectDelay = 30
	}

	if config.IrcPort == 0 {
		config.IrcPort = 6697
	}

	if config.KeepAlive == 0 {
		config.KeepAlive = 600
	}

	if config.MemoryLimit == 0 {
		config.MemoryLimit = 20
	}

	if config.PingDelay == 0 {
		config.PingDelay = 20
	}

	if config.PingTimeout == 0 {
		config.PingTimeout = 20
	}

	if config.OllamaMirostatEta == 0 {
		config.OllamaMirostatEta = 0.1
	}

	if config.OllamaMirostatTau == 0 {
		config.OllamaMirostatTau = 5.0
	}

	if config.OllamaNumCtx == 0 {
		config.OllamaNumCtx = 4096
	}

	if config.OllamaRepeatLastN == 0 {
		config.OllamaRepeatLastN = 64
	}

	if config.OllamaRepeatPenalty == 0 {
		config.OllamaRepeatPenalty = 1.1
	}

	if config.OllamaSeed == 0 {
		config.OllamaSeed = 42
	}

	if config.OllamaNumPredict == 0 {
		config.OllamaNumPredict = -1
	}

	if config.TopK == 0 {
		config.TopK = 40
	}

	if config.TopP == 0.0 {
		config.TopP = 0.9
	}

	if config.OllamaMinP == 0 {
		config.OllamaMinP = 0.05
	}

	if config.Temperature == 0 {
		config.Temperature = 0.7
	}

	if config.IrcBackOffMaxInterval == 0 {
		config.IrcBackOffMaxInterval = 500
	}

	if config.IrcBackOffRandomizationFactor == 0 {
		config.IrcBackOffRandomizationFactor = 0.5
	}

	if config.IrcBackOffMultiplier == 0 {
		config.IrcBackOffMultiplier = 1.5
	}

	if config.IrcBackOffMaxInterval == 0 {
		config.IrcBackOffMaxInterval = 60
	}

	if config.DbBackOffMaxInterval == 0 {
		config.DbBackOffMaxInterval = 500
	}

	if config.DbBackOffRandomizationFactor == 0 {
		config.DbBackOffRandomizationFactor = 0.5
	}

	if config.DbBackOffMultiplier == 0 {
		config.DbBackOffMultiplier = 1.5
	}

	if config.DbBackOffMaxInterval == 0 {
		config.DbBackOffMaxInterval = 60
	}

	if config.OllamaThink == "" {
		config.OllamaThink = "false"
	}
}
