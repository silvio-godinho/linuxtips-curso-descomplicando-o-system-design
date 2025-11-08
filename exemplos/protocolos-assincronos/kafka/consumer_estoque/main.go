package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

const groupID = "estoque-consumer-group"

// Estrutura de exemplo para mensagens (mesma do producer)
type Pedido struct {
	ID         string    `json:"id"`
	Cliente    string    `json:"cliente"`
	Produto    string    `json:"produto"`
	Quantidade int       `json:"quantidade"`
	Valor      float64   `json:"valor"`
	Timestamp  time.Time `json:"timestamp"`
}

// Consumer Handler
type ConsumerHandler struct {
	ready chan bool
}

func (h *ConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *ConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			// Deserializa a mensagem
			var pedido Pedido
			if err := json.Unmarshal(message.Value, &pedido); err != nil {
				log.Printf("Erro ao deserializar mensagem: %v", err)
				session.MarkMessage(message, "")
				continue
			}

			log.Printf("| Consumer Group: %s | Reservando item no estoque: %s | Quantidade: %d | Topic: %s | Partition: %d | Offset: %d",
				groupID, pedido.Produto, pedido.Quantidade, message.Topic, message.Partition, message.Offset)

			// Marca a mensagem como processada
			session.MarkMessage(message, "")

			// log.Println("Pedido processado com sucesso!")
			// log.Println()

		case <-session.Context().Done():
			return nil
		}
	}
}

func main() {
	// Configuração do Kafka
	config := sarama.NewConfig()
	config.Version = sarama.V3_5_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second

	// Lista de brokers
	brokers := []string{"localhost:9092"}
	topics := []string{"pedidos"}

	// Cria o consumer group
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		log.Fatalf("Erro ao criar consumer group: %v", err)
	}
	defer client.Close()

	handler := &ConsumerHandler{
		ready: make(chan bool),
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			if err := client.Consume(ctx, topics, handler); err != nil {
				log.Printf("Erro no consumer: %v", err)
			}

			// Verifica se o contexto foi cancelado
			if ctx.Err() != nil {
				return
			}

			handler.ready = make(chan bool)
		}
	}()

	<-handler.ready
	log.Println("Consumer iniciado com sucesso!")
	log.Printf("Group ID: %s", groupID)
	log.Printf("Tópicos: %v", topics)
	log.Println("Aguardando mensagens...")
	log.Println()

	// Aguarda sinal de interrupção
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		log.Println("Contexto cancelado")
	case <-sigterm:
		log.Println("Sinal de interrupção recebido")
	}

	cancel()
	wg.Wait()

	log.Println("Consumer encerrado")
}
