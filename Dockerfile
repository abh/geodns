FROM golang:1.10.1-alpine3.7 as compiler

RUN apk add --no-cache git
WORKDIR /go/src/github.com/abh/geodns

ENV CGO_ENABLED=0

ADD . .

RUN go get -d -v ./...
RUN go build  -o /geodns


FROM scratch
COPY --from=compiler /geodns /geodns
ENTRYPOINT ["/geodns"]
