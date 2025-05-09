class AgentRegistry:
    def __init__(self):
        self.registry = {}

    def __call__(self, func):
        self.registry[func.__name__] = func
        print(f"Registered agent: {func.__name__}")
        return func


agentRegistry = AgentRegistry()
