FROM golang:1.12.3-alpine3.9 as build

RUN apk add --no-cache git
WORKDIR /go/src/github.com/abh/geodns

ENV CGO_ENABLED=0

ADD . .

RUN go get -d -v ./...
RUN go build  -o /geodns


FROM scratch
COPY --from=build /geodns /geodns
ENTRYPOINT ["/geodns"]
