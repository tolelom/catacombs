FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o catacombs .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/catacombs .
EXPOSE 2222
CMD ["./catacombs"]
