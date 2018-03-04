FROM golang:1.10-alpine as builder
WORKDIR /go/src/github.com/marpio/mirror/
COPY . . 
RUN cd cmd/mirror-web && go build -o app

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
RUN mkdir -p /usr/app
COPY --from=builder /go/src/github.com/marpio/mirror/cmd/mirror-web/ /usr/app/
RUN chown -R nobody:nogroup /usr/app  
USER nobody
WORKDIR /usr/app
CMD ["./app"]  