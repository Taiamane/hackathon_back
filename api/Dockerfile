FROM golang:1.18 as build
WORKDIR /app

COPY . .
RUN go mod init api \
 && go mod tidy
EXPOSE 8080


RUN go build main.go

CMD ["./main"]