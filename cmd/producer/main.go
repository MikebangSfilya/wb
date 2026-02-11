package main

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/MikebangSfilya/wb/internal/config"
	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/MikebangSfilya/wb/internal/transport/kafka"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	prod, err := kafka.NewProducer(context.Background(), cfg.Kafka.Brokers, cfg.Kafka.Topic)
	if err != nil {
		panic(err)
	}
	defer prod.Close()

	createdAt := time.Now().UTC()
	mainOrder := model.Order{
		OrderUID:          "b563feb7b2b84b6test",
		TrackNumber:       "WBILMTESTTRACK",
		Entry:             "WBIL",
		Locale:            "en",
		InternalSignature: "",
		CustomerID:        "test",
		DeliveryService:   "meest",
		Shardkey:          "9",
		SmID:              99,
		DateCreated:       createdAt,
		OofShard:          "1",
		Delivery: model.Delivery{
			Name:    "Test Testov",
			Phone:   "+9720000000",
			Zip:     "2639809",
			City:    "Kiryat Mozkin",
			Address: "Ploshad Mira 15",
			Region:  "Kraiot",
			Email:   "test@gmail.com",
		},
		Payment: model.Payment{
			Transaction:  "b563feb7b2b84b6test",
			RequestID:    "",
			Currency:     "USD",
			Provider:     "wbpay",
			Amount:       1817,
			PaymentDt:    1637907727,
			Bank:         "alpha",
			DeliveryCost: 1500,
			GoodsTotal:   317,
			CustomFee:    0,
		},
		Items: []model.Item{
			{
				ChrtID:      9934930,
				TrackNumber: "WBILMTESTTRACK",
				Price:       453,
				Rid:         "ab4219087a764ae0btest",
				Name:        "Mascaras",
				Sale:        30,
				Size:        "0",
				TotalPrice:  317,
				NmID:        2389212,
				Brand:       "Vivienne Sabo",
				Status:      202,
			},
		},
	}
	sendOrder(prod, mainOrder)

	for i := 1; i <= 12; i++ {
		simpleOrder := mainOrder

		simpleID := strconv.Itoa(i)
		simpleOrder.OrderUID = simpleID
		simpleOrder.TrackNumber = "TRACK-" + simpleID

		simpleOrder.Payment.Transaction = "trans-" + simpleID

		simpleOrder.Payment.Amount += i
		sendOrder(prod, simpleOrder)
	}

	log.Println("All orders sent!")
}

func sendOrder(prod *kafka.Producer, order model.Order) {
	payload, err := json.Marshal(order)
	if err != nil {
		log.Printf("JSON Error: %v", err)
		return
	}

	const maxRetries = 5

	for i := 1; i <= maxRetries; i++ {
		err = prod.SendMessage(context.Background(), order.OrderUID, payload)
		if err == nil {
			log.Printf("Sent order: %s", order.OrderUID)
			return
		}
		log.Printf("Attempt %d/%d, err %v. Retrying...", i, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	log.Printf("Failed to send order %s after %d attempts", order.OrderUID, maxRetries)
}
