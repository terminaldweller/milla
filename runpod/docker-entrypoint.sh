#!/usr/bin/env sh

ollama serve 2>&1 | tee /var/log/ollama.log &
ollama pull mistral

python3 /app/main.py
