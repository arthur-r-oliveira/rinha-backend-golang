FROM golang:1.22-alpine

RUN apk --no-cache add wget

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . ./

# Ensure dependencies are in sync
RUN go mod tidy

RUN go build -o /rinha-backend-go .

EXPOSE 8080

CMD ["/rinha-backend-go"]
