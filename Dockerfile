FROM golang:latest
WORKDIR /app
COPY . /app
RUN go get github.com/joho/godotenv
RUN go get github.com/streadway/amqp
RUN go get github.com/gorilla/websocket
RUN go get github.com/go-sql-driver/mysql
RUN go get github.com/aws/aws-sdk-go
RUN go get github.com/zbindenren/logrus_mail
CMD ["go", "run", "main.go", "api.go", "manager.go", "server.go", "sql.go", "publisher.go"]
EXPOSE 80

