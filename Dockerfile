FROM golang:1.21 as gobuild

RUN update-ca-certificates

WORKDIR /go/src/app

COPY go.mod .
COPY go.sum .

ENV GO111MODULE=on
RUN go mod download
RUN go mod verify

COPY ./ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/server ./cmds/server

FROM gcr.io/distroless/static

COPY --from=gobuild /go/bin/server /go/bin/server
EXPOSE 8080

ENTRYPOINT ["/go/bin/server"]
