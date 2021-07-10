FROM golang:latest

WORKDIR /go/src/github.com/mayankkumar2/Alt-Reality-backend
COPY go.mod .
RUN go mod tidy
COPY . .
RUN go build -o app
CMD ["app"]