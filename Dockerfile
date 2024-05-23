FROM golang:1.21.9 as build

WORKDIR /rolodex

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

EXPOSE 1337

ENTRYPOINT ["go", "run", "main.go"]