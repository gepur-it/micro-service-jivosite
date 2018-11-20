FROM golang:latest
WORKDIR /app
COPY . /app
RUN go get github.com/joho/godotenv
RUN go get github.com/streadway/amqp
RUN go get github.com/gorilla/websocket
RUN go get github.com/go-sql-driver/mysql
RUN go get github.com/aws/aws-sdk-go
CMD ["go", "run", "main.go"]
EXPOSE 80

