FROM alpine:3.19 as builder
RUN apk update && apk upgrade && \
      apk add go git
WORKDIR /milla
COPY go.sum go.mod /milla/
RUN go mod download
COPY *.go /milla/
RUN go build

FROM alpine:3.19
COPY --from=builder /milla/milla /milla/
ENTRYPOINT ["/milla/milla"]
