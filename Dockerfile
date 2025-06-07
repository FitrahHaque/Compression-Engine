FROM golang:1.24-alpine AS build
# RUN apk add --no-cache git \
#     && go install github.com/githubnemo/CompileDaemon@latest
WORKDIR /app
COPY go.* ./
RUN go mod download
# CMD ["CompileDaemon", "-directory=.", "-recursive", "-command=go run main.go"]
COPY . .
RUN go build -o engine .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/ .
CMD [ "./engine" ]
