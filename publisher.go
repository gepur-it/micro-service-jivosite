package main

import (
	"github.com/streadway/amqp"
	"time"
)

func publishToErp(message []byte) error {
	var err error

	err = AMQPChannel.Publish(
		"",
		"chat_to_erp_handle_messages",
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Transient,
			ContentType:  "application/json",
			Body:         message,
			Timestamp:    time.Now(),
		})

	if err != nil {
		return err
	}

	return nil
}
