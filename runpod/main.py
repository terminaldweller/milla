#!/usr/bin/env python
"""ollama-runpod endpoint"""


import runpod
import requests


def ollama(job):
    """the endpoint handler"""
    headers = {"Content-Type": "application/json"}
    data = {
        "model": job["model"],
        "system": job["prompt_system"],
        "prompt": job["prompt_user"] + job["content"],
        "stream": False,
        "format": "json",
        "options": {
            "temperature": job["temp"],
        },
    }

    response = requests.post(
        "http://127.0.0.1:11434/api/generate", data=data, headers=headers, timeout=10
    )

    return response


def main() -> None:
    """entrypoint"""
    runpod.serverless({"handler": ollama})
