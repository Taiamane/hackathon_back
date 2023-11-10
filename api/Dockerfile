FROM golang:1.18 as build
WORKDIR /app
COPY .. .
EXPOSE 8080
RUN go build /api/main.go
CMD ["/api/main"]