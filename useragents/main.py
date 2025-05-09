#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
API Server for custom user agents
"""

import argparse
import importlib.util
import os
import sys

from src.models import AgentRequest, AgentResponse
from src.registry import agentRegistry

from agents import Runner, Agent
import fastapi
from fastapi.encoders import jsonable_encoder
from fastapi.responses import JSONResponse
import uvicorn


class ArgParser:
    """
    Argument parser for command line arguments.
    """

    def __init__(self):
        parser = argparse.ArgumentParser()
        parser.add_argument(
            "--plugin-dir",
            type=str,
            help="Directory containing plugins",
            default="/useragent/src/custom_agents",
        )
        parser.add_argument(
            "--port", type=int, help="Port to run the server on", default=443
        )
        parser.add_argument(
            "--address", type=str, help="which address to bind to", default="0.0.0.0"
        )
        self.args = parser.parse_args()


def load_plugins(plugin_dir: str):
    """
    Load all Python files in the specified directory as modules.
    Args:
        plugin_dir (str): Directory containing the plugin files.
    """
    for filename in os.listdir(plugin_dir):
        if filename.endswith(".py") and filename != "__init__.py":
            filepath = os.path.join(plugin_dir, filename)
            print(filepath)
            module_name = filename[:-3]
            spec = importlib.util.spec_from_file_location(module_name, filepath)
            if spec and spec.loader:
                module = importlib.util.module_from_spec(spec)
                sys.modules[module_name] = module
                spec.loader.exec_module(module)
                print(f"Loaded module: {module_name} from {filepath}")
            else:
                print(f"Warning: Could not load module {module_name} from {filepath}")


def get_agent_generator(agent_name: str) -> Agent:
    """
    Find and return the agent generator function from the custom agent registry
    """
    for k, v in agentRegistry.registry.items():
        print(k, v)
    try:
        agent = agentRegistry.registry[agent_name]
    except KeyError:
        raise ValueError(f"Agent {agent_name} not found in registry.")

    return agent


class AgentRunner:
    """
    Class to run the agent with the provided request.
    """

    def __init__(self, agent_request):
        self.agent_request = agent_request

    async def run(self) -> str:
        print(
            f"Running agent {self.agent_request.agent_name} with instructions: {self.agent_request.instructions} with query: {self.agent_request.query}"
        )

        try:
            agent = get_agent_generator(self.agent_request.agent_name)(
                self.agent_request
            )
        except ValueError as e:
            print(f"Error: {e}")
            return str(e)

        result = await Runner.run(agent, self.agent_request.query)
        print(result.final_output_as(str))

        return result.final_output_as(str)


class APIServer:
    """
    The API server
    """

    def __init__(self):
        self.app = fastapi.FastAPI()
        self.router = fastapi.APIRouter()
        self.router.add_api_route(
            "/api/v1/agent", self.agent_handler, methods=["POST"], tags=[]
        )
        self.agent_registry = {}

        self.app.include_router(self.router)

    async def agent_handler(self, agent_request: AgentRequest) -> fastapi.Response:
        print(f"Received request: {agent_request}")
        response = await AgentRunner(agent_request).run()

        result = AgentResponse(agent_name=agent_request.agent_name, response=response)

        print(f"Response: {result}")

        return JSONResponse(
            content=jsonable_encoder(result),
            status_code=200,
        )


def main():
    argparser = ArgParser()
    load_plugins(argparser.args.plugin_dir)
    app = APIServer().app
    uvicorn.run(
        app,
        host=argparser.args.address,
        port=argparser.args.port,
        ssl_keyfile="./server.key",
        ssl_certfile="./server.cert",
    )


if __name__ == "__main__":
    main()
