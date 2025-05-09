FROM python:3.13-slim AS python-base
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=off \
    PIP_DISABLE_PIP_VERSION_CHECK=on \
    PIP_DEFAULT_TIMEOUT=100 \
    POETRY_HOME="/poetry" \
    POETRY_VIRTUALENVS_IN_PROJECT=true \
    POETRY_NO_INTERACTION=1 \
    PYSETUP_PATH="/useragent" \
    VENV_PATH="/useragent/.venv"
ENV PATH="$POETRY_HOME/bin:$VENV_PATH/bin:$PATH"

FROM python-base AS builder-base
ENV POETRY_VERSION=2.1.1
RUN apt-get update && apt-get install -y --no-install-recommends curl build-essential
RUN curl -sSL https://install.python-poetry.org | python -
WORKDIR $PYSETUP_PATH
COPY ./pyproject.toml ./
RUN poetry install --without dev

FROM alpine:3.21 AS certbuilder
RUN apk add openssl
WORKDIR /certs
RUN openssl req -nodes -new -x509 -subj="/C=/ST=/L=/O=/CN=useragents" -keyout server.key -out server.cert

FROM python-base AS production
RUN apt-get update && apt-get install --no-install-recommends -y poppler-utils python3-magic
COPY --from=certbuilder /certs/ $PYSETUP_PATH/
ENV FASTAPI_ENV=production
COPY --from=builder-base $VENV_PATH $VENV_PATH
COPY ./src $PYSETUP_PATH/src
COPY ./main.py $PYSETUP_PATH/main.py
WORKDIR $PYSETUP_PATH
