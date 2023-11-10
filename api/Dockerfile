FROM golang:1.18 as build
WORKDIR /app
COPY . .
EXPOSE 8080
COPY go.mod go.sum ./
RUN go mod download
RUN go build main.go

CMD ["./main"]