FROM golang:1.21.2 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .

####

FROM alpine:latest  

# Set these to override the default values
#ENV CANONICAL_HOST= (defaults to njump.me)
#ENV PORT= (defaults to 2999)

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .

CMD ["./main"] 

