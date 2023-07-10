FROM golang:1.20.5-alpine as build

RUN apk add --no-cache git tar

WORKDIR /go/src/github.com/abh/geodns

ENV CGO_ENABLED=0

ADD applog/ applog/
ADD countries/ countries/
ADD health/ health/
ADD monitor/ monitor/
ADD querylog/ querylog/
ADD server/ server/
ADD targeting/ targeting/
ADD typeutil/ typeutil/
ADD zones/ zones/
ADD service/ service/
ADD .git/ .git/
ADD *.go build ./

RUN ./build
RUN ls -l
RUN ls -l dist

RUN ln dist/* /

FROM scratch
COPY --from=build /geodns-linux-amd64 /geodns
ENTRYPOINT ["/geodns"]
