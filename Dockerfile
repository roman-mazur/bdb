FROM balenalib/raspberrypi3-golang:latest-build AS build

RUN install_packages libusb-1.0-0-dev

WORKDIR /go/src/rmazur.io/bdb
COPY . .
RUN go get -d -v ./...
RUN go build -o bdbd rmazur.io/bdb/cmd/bdbd && ls -la

CMD ["/go/src/rmazur.io/bdb/entry.sh"]
