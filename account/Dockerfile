# build stage
FROM golang:1.14.6-alpine AS build-env
#RUN apk --no-cache add build-base git bzr mercurial gcc
ADD . /src
RUN ls -l /src && cd /src/cmd/account && go build -o goapp

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /src/cmd/account/goapp /app/
ENTRYPOINT ./goapp
