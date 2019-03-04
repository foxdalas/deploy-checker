FROM golang:alpine as build

WORKDIR $GOPATH/src/github.com/foxdalas/deploy-checker
COPY . .

RUN apk --no-cache add git
RUN go get -u github.com/golang/dep/cmd/dep
RUN dep check || dep ensure --vendor-only -v
RUN go build -o /go/bin/deploy-checker .

FROM alpine:3.9
RUN apk --no-cache add ca-certificates git
COPY --from=build /go/bin/deploy-checker /app/
