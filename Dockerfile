FROM golang:1.13.5-alpine3.10 as build

RUN apk add --no-cache git tar

WORKDIR /go/src/github.com/abh/geodns

ENV CGO_ENABLED=0

ADD vendor/ vendor/
ADD applog/ applog/
ADD countries/ countries/
ADD geodns-logs/ geodns-logs/
ADD health/ health/
ADD monitor/ monitor/
ADD querylog/ querylog/
ADD server/ server/
ADD targeting/ targeting/
ADD typeutil/ typeutil/
ADD zones/ zones/
ADD service/ service/
ADD service-logs/ service-logs/
ADD .git/ .git/
ADD *.go build ./

RUN ./build
RUN ls -l
RUN ls -l dist

RUN ln dist/* /

FROM scratch
COPY --from=build /geodns-linux-amd64 /geodns
COPY --from=build /geodns-logs-linux-amd64 /geodns-logs
ENTRYPOINT ["/geodns"]
