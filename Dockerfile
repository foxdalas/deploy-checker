FROM golang:onbuild

RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go get -u github.com/golang/dep/cmd/dep
RUN dep ensure --vendor-only -v
RUN go build -o deploy-checker .
RUN ls | grep -v deploy-checker | xargs rm -rf
RUN rm -rf /go/src

ENTRYPOINT ["go-wrapper", "run"]