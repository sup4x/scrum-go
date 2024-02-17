FROM golang:1.22
LABEL authors="sup4x"
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN go mod tidy
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /scrum-go
CMD ["/scrum-go"]