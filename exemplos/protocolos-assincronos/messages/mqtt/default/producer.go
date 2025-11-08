package main

import (
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("[PRODUCER] Conectado ao broker MQTT")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("[PRODUCER] Conexão perdida: %v\n", err)
}

func main() {
	broker := "tcp://localhost:1883"

	topic := "linuxtips/iot"

	clientID := fmt.Sprintf("producer-%d", time.Now().Unix())

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	opts.SetAutoReconnect(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	// Aguarda um pouco para garantir que o consumer está pronto
	time.Sleep(3 * time.Second)

	// Publica mensagens a cada 5 segundos
	counter := 1
	for {
		message := fmt.Sprintf("Mensagem #%d - %s", counter, time.Now().Format(time.RFC3339))

		token := client.Publish(topic, 1, false, message)
		token.Wait()

		if token.Error() != nil {
			fmt.Printf("[PRODUCER] Erro ao publicar: %v\n", token.Error())
		} else {
			fmt.Printf("[PRODUCER] Mensagem publicada: %s\n", message)
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}
