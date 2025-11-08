package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

// Estrutura de exemplo para mensagens
type Pedido struct {
	ID         string    `json:"id"`
	Cliente    string    `json:"cliente"`
	Produto    string    `json:"produto"`
	Quantidade int       `json:"quantidade"`
	Valor      float64   `json:"valor"`
	Timestamp  time.Time `json:"timestamp"`
}

func main() {
	// Configuração do Kafka
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.WaitForAll // Espera confirmação de todas as réplicas
	config.Producer.Retry.Max = 5
	config.Producer.Compression = sarama.CompressionSnappy
	config.Version = sarama.V3_5_0_0

	brokers := []string{"localhost:9092"}

	// Cria o produtor
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatalf("Erro ao criar produtor: %v", err)
	}
	defer producer.Close()

	log.Println("Producer iniciado com sucesso!")
	log.Println("Enviando mensagens para o tópico 'pedidos'...")

	// Simula envio de mensagens periodicamente
	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	contador := 1

	for {
		select {
		case <-ticker.C:
			// Cria um pedido de exemplo
			pedido := Pedido{
				ID:         fmt.Sprintf("PEDIDO-%05d", contador),
				Cliente:    fmt.Sprintf("Cliente-%d", contador%10+1),
				Produto:    getProdutoAleatorio(contador),
				Quantidade: (contador % 5) + 1,
				Valor:      float64((contador%10)+1) * 29.90,
				Timestamp:  time.Now(),
			}

			// Serializa para JSON
			mensagemJSON, err := json.Marshal(pedido)
			if err != nil {
				log.Printf("Erro ao serializar mensagem: %v", err)
				continue
			}

			// Cria a mensagem Kafka
			msg := &sarama.ProducerMessage{
				Topic: "pedidos",
				Key:   sarama.StringEncoder(pedido.ID),
				Value: sarama.ByteEncoder(mensagemJSON),
				Headers: []sarama.RecordHeader{
					{
						Key:   []byte("producer"),
						Value: []byte("go-producer-exemplo"),
					},
					{
						Key:   []byte("version"),
						Value: []byte("1.0"),
					},
				},
			}

			// Envia a mensagem
			partition, offset, err := producer.SendMessage(msg)
			if err != nil {
				log.Printf("Erro ao enviar mensagem: %v", err)
			} else {
				log.Printf("Mensagem enviada | Pedido: %s | Partition: %d | Offset: %d | Cliente: %s | Produto: %s",
					pedido.ID, partition, offset, pedido.Cliente, pedido.Produto)
			}

			contador++

		case <-context.Background().Done():
			log.Println("Producer encerrado")
			return
		}
	}
}

// Função auxiliar para variar os produtos
func getProdutoAleatorio(seed int) string {
	produtos := []string{
		"Notebook Dell",
		"Mouse Logitech",
		"Teclado Mecânico",
		"Monitor LG 27",
		"Webcam Logitech",
		"Headset Gamer",
		"SSD 1TB",
		"Memória RAM 16GB",
		"Placa de Vídeo RTX",
		"Mousepad Gamer",
	}
	return produtos[seed%len(produtos)]
}
