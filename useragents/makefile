.PHONY: d_test d_down d_build help up down test log

IMAGE_NAME=useragents

up: d_test

down: d_down

log:
	nqtail

test:
	@curl -k -X POST -H 'Content-Type: application/json' --data '{"agent_name": "web_search_tool", "instructions":"you are a web search bot", "query": "make me a cryptocurrency news digest from the news for today please.  give me the references for the news you mention."}' https://127.0.0.1:9910/api/v1/agent

d_test:
	nq docker compose -f ./docker-compose-uas.yaml up --build

d_down:
	docker compose -f ./docker-compose-uas.yaml down

d_build: d_build_regular

d_build_regular:
	docker build -t $(IMAGE_NAME)-f ./Dockerfile .

help:
	@echo "up"
	@echo "down"
	@echo "test"
	@echo "d_test"
	@echo "d_down"
	@echo "d_build"
