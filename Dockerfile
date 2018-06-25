FROM golang:onbuild

RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go build -o deploy-checker .
RUN ls | grep -v deploy-checker | xargs rm -rf
RUN rm -rf /go/src

ENTRYPOINT ["go-wrapper", "run"]