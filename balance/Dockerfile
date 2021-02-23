# build stage
FROM golang:1.14.6-alpine AS build-env
ADD ./account /account
ADD ./balance /balance
RUN cd /balance/cmd/balance && go build -o goapp

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /balance/cmd/balance/goapp /app/
ENTRYPOINT ./goapp
