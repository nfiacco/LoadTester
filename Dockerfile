FROM golang:1.21

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . ./

RUN go build -mod=readonly -v -o loadtest cmd/main.go

ENTRYPOINT ["./loadtest"]