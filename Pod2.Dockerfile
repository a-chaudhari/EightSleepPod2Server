FROM golang:1.25-alpine
LABEL authors="amitchaudhari"

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -v -o /usr/local/bin/ ./...

WORKDIR /usr/local/bin

EXPOSE 5683/tcp
EXPOSE 1337/tcp

CMD ["EightSleepServer"]