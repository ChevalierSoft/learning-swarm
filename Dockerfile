FROM golang:alpine3.20

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/app .

EXPOSE 45000

CMD ["/app/app"]
