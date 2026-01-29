FROM golang:1.25-alpine
LABEL authors="amitchaudhari"

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -v -o /usr/local/bin/ ./...

WORKDIR /usr/local/bin

EXPOSE 5683/tcp
EXPOSE 1337/tcp

USER 1000:1000

CMD ["EightSleepServer"]