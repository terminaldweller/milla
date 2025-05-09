import pydantic


class AgentRequest(pydantic.BaseModel):
    agent_name: str
    instructions: str
    query: str


class AgentResponse(pydantic.BaseModel):
    agent_name: str
    response: str
