FROM golang:1.9-alpine as builder
WORKDIR /go/src/github.com/marpio/img-store/
COPY . . 
RUN cd cmd/pics-web && go build -o app

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
RUN mkdir -p /usr/app
COPY --from=builder /go/src/github.com/marpio/img-store/cmd/pics-web/ /usr/app/
RUN chown -R nobody:nogroup /usr/app  
USER nobody
WORKDIR /usr/app
CMD ["./app"]  