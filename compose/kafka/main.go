package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/13 18:47:32
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/13 18:47:32
 */

func main() {
	Product()
	fmt.Println("********************************")
	Consumer()
}

func Product() {
	// to produce messages
	topic := "my-topic"
	partition := 0

	conn, err := kafka.Dial("tcp", "localhost:9092")

	//conn, err := kafka.DialLeader(context.Background(), "tcp", "localhost:9092", topic, partition)
	if err != nil {
		log.Fatal("failed to dial leader:", err)
	}

	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = conn.WriteMessages(
		kafka.Message{Topic: topic, Partition: partition, Value: []byte("one!")},
		kafka.Message{Topic: topic, Partition: partition, Value: []byte("two!")},
		kafka.Message{Topic: topic, Partition: partition, Value: []byte("three!")},
	)
	if err != nil {
		log.Fatal("failed to write messages:", err)
	}

	if err := conn.Close(); err != nil {
		log.Fatal("failed to close writer:", err)
	}
}

func Consumer() {
	// to consume messages
	topic := "my-topic"
	partition := 0

	// make a new reader that consumes from topic-A, partition 0, at offset 42
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{"localhost:9092"},
		Topic:     topic,
		Partition: partition,
		MaxBytes:  10e6, // 10MB
		GroupID:   "0",
	})

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			break
		}
		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
	}

	if err := r.Close(); err != nil {
		log.Fatal("failed to close reader:", err)
	}
}
