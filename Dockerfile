FROM golang:alpine as build

WORKDIR $GOPATH/src/github.com/foxdalas/deploy-checker
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN apk --no-cache add git
RUN go build -o /go/bin/deploy-checker .

FROM alpine:3.10
RUN apk --no-cache add ca-certificates git
COPY --from=build /go/bin/deploy-checker /app/
