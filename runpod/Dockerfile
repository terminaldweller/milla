FROM ollama/ollama:0.1.23 as python-base
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=off \
    PIP_DISABLE_PIP_VERSION_CHECK=on \
    PIP_DEFAULT_TIMEOUT=100 \
    POETRY_HOME="/poetry" \
    POETRY_VIRTUALENVS_IN_PROJECT=true \
    POETRY_NO_INTERACTION=1 \
    PYSETUP_PATH="/app" \
    VENV_PATH="/app/.venv"
ENV PATH="$POETRY_HOME/bin:$VENV_PATH/bin:$PATH"

FROM python-base as builder-base
ENV POETRY_VERSION=1.7.1
RUN apt update && apt install -y --no-install-recommends curl build-essential python3 python3-pip
RUN curl -sSL https://install.python-poetry.org | python3 -
WORKDIR $PYSETUP_PATH
COPY ./pyproject.toml ./
COPY ./poetry.lock ./
RUN poetry install --no-dev

FROM alpine:3.18 AS certbuilder
RUN apk add openssl
WORKDIR /certs
RUN openssl req -nodes -new -x509 -subj="/C=US/ST=Denial/L=springfield/O=Dis/CN=localhost" -keyout server.key -out server.cert

FROM python-base as production
COPY --from=certbuilder /certs/ /certs
COPY --from=builder-base $VENV_PATH $VENV_PATH
COPY ./main.py $PYSETUP_PATH/main.py
COPY ./docker-entrypoint.sh /app/docker-entrypoint.sh
ENTRYPOINT ["/app/docker-entrypoint.sh"]
