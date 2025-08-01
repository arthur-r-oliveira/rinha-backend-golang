FROM golang:1.22-alpine

RUN apk --no-cache add wget

WORKDIR /app

COPY api/go.mod .
COPY api/go.sum .

RUN go mod download

COPY api/ ./

RUN go build -o /rinha-backend-go .

EXPOSE 8080

CMD ["/rinha-backend-go"]
