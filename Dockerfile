FROM golang:1.22-alpine3.20 as builder
WORKDIR /milla
COPY go.sum go.mod /milla/
RUN go mod download
COPY *.go /milla/
RUN go build

FROM alpine:3.20
ENV HOME /home/user
RUN set -eux; \
  adduser -u 1001 -D -h "$HOME" user; \
  mkdir "$HOME/.irssi"; \
  chown -R user:user "$HOME"
COPY --from=builder /milla/milla "$HOME/milla"
RUN chown user:user "$HOME/milla"
ENTRYPOINT ["home/user/milla"]
