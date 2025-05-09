from agents import Agent, WebSearchTool
from src.current_time import fetch_date
from src.models import AgentRequest
from src.registry import agentRegistry


@agentRegistry
def web_search_tool(agent_request: AgentRequest) -> Agent:
    tools = [WebSearchTool(), fetch_date]

    agent = Agent(
        name=agent_request.agent_name,
        instructions=agent_request.instructions,
        tools=tools,
    )

    return agent
