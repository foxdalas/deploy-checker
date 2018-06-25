FROM golang:onbuild

RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go build .
RUN ls | grep -v deploy-checker | xargs rm -rf

ENTRYPOINT ["go-wrapper", "run"]