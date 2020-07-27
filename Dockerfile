FROM golang as prepare
COPY go.mod go.sum /goapp/
WORKDIR /goapp/
RUN go mod download

FROM golang as build
ENV CGO_ENABLED=0
ARG REVISION=${REVISION:-""}
ENV REVISION=${REVISION}
COPY --from=prepare /go/pkg/mod /go/pkg/mod
COPY . /goapp
WORKDIR /goapp/
RUN go mod vendor
RUN go build -o dist/geodns \
      -mod=vendor \
      -trimpath \
      -ldflags "-X main.gitVersion=$REVISION -X main.buildTime=`TZ=UTC date "+%Y-%m-%dT%H:%MZ"`" \
      -v && \
      (cd geodns-logs && go build -trimpath -mod=vendor -v -o ../dist/geodns-logs && cd ..)

FROM scratch
COPY --from=build /goapp/dist/geodns /geodns
COPY --from=build /goapp/dist/geodns-logs /geodns-logs
RUN mkdir ./dns/
ENTRYPOINT ["/geodns"]
