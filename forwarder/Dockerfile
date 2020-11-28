# build stage
FROM golang:1.14.6-alpine AS build-env
ADD . /src
RUN cd /src/cmd/forwarder && go build -o goapp

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /src/cmd/forwarder/goapp /app/
ENTRYPOINT ./goapp
