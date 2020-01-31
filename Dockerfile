FROM golang:1.13.5-alpine3.10 AS godev

RUN apk update && apk add --no-cache ca-certificates && apk upgrade && apk add git

WORKDIR /jira-sync

COPY . .
ENV GO111MODULE=on
ENV GOSUMDB=off
ENV GOPROXY=direct

RUN go build -o jira-sync .

FROM alpine:3.9

COPY VERSION .

RUN apk update && apk add --no-cache ca-certificates && apk upgrade

COPY --from=godev ./jira-sync/jira-sync /jira-sync