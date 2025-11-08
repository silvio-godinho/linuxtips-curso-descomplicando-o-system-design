package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

// Estrutura de exemplo para mensagens
type Pedido struct {
	ID               string    `json:"id"`
	ClienteID        string    `json:"cliente_id"`
	ClienteNome      string    `json:"cliente_nome"`
	Produto          string    `json:"produto"`
	Quantidade       int       `json:"quantidade"`
	Valor            float64   `json:"valor"`
	Timestamp        time.Time `json:"timestamp"`
	SequenciaCliente int       `json:"sequencia_cliente"`
}

// Definindo 3 clientes com UUIDs fixos
var clientes = []struct {
	ID   string
	Nome string
}{
	{
		ID:   "550e8400-e29b-41d4-a716-446655440001",
		Nome: "Matheus Scarpato Fidelis",
	},
	{
		ID:   "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		Nome: "Renan Martins",
	},
	{
		ID:   "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		Nome: "Pedro Oliveira",
	},
	{
		ID:   "7c9e6679-7425-40de-944b-s123fc1f90ae7",
		Nome: "Tarsila Bianca",
	},
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

	// IMPORTANTE: Define o particionador para usar a chave
	config.Producer.Partitioner = sarama.NewHashPartitioner

	brokers := []string{"localhost:9092"}

	// Cria o produtor
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatalf("Erro ao criar produtor: %v", err)
	}
	defer producer.Close()

	log.Println("===========================================")
	log.Println("Producer com Partition Key iniciado!")
	log.Println("===========================================")
	log.Println("Clientes cadastrados:")
	for i, cliente := range clientes {
		log.Printf("  %d. %s (ID: %s)", i+1, cliente.Nome, cliente.ID)
	}
	log.Println("===========================================")
	log.Println("Enviando mensagens para o tópico 'pedidos-particionados'...")
	log.Println("A partition key será o ID do cliente")
	log.Println("Mensagens do mesmo cliente sempre vão para a mesma partição")
	log.Println("===========================================")
	log.Println()

	// Contador de sequência por cliente
	sequenciaCliente := make(map[string]int)

	// Simula envio de mensagens periodicamente
	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	contador := 1
	rand.Seed(time.Now().UnixNano())

	for range ticker.C {
		// Seleciona um cliente aleatório
		cliente := clientes[rand.Intn(len(clientes))]

		// Incrementa a sequência do cliente
		sequenciaCliente[cliente.ID]++

		// Cria um pedido de exemplo
		pedido := Pedido{
			ID:               uuid.New().String(),
			ClienteID:        cliente.ID,
			ClienteNome:      cliente.Nome,
			Produto:          getProdutoAleatorio(),
			Quantidade:       rand.Intn(5) + 1,
			Valor:            float64(rand.Intn(500)+50) + 0.99,
			Timestamp:        time.Now(),
			SequenciaCliente: sequenciaCliente[cliente.ID],
		}

		// Serializa para JSON
		mensagemJSON, err := json.Marshal(pedido)
		if err != nil {
			log.Printf("Erro ao serializar mensagem: %v", err)
			continue
		}

		// Cria a mensagem Kafka
		// IMPORTANTE: A Key é o ID do Cliente - isso garante que mensagens
		// do mesmo cliente sempre vão para a mesma partição
		msg := &sarama.ProducerMessage{
			Topic: "pedidos-particionados",
			Key:   sarama.StringEncoder(pedido.ClienteID), // PARTITION KEY = ClienteID
			Value: sarama.ByteEncoder(mensagemJSON),
			Headers: []sarama.RecordHeader{
				{
					Key:   []byte("producer"),
					Value: []byte("partition-key-exemplo"),
				},
				{
					Key:   []byte("cliente_id"),
					Value: []byte(pedido.ClienteID),
				},
				{
					Key:   []byte("cliente_nome"),
					Value: []byte(pedido.ClienteNome),
				},
			},
		}

		// Envia a mensagem
		partition, offset, err := producer.SendMessage(msg)
		if err != nil {
			log.Printf("Erro ao enviar mensagem: %v", err)
		} else {
			log.Printf("Msg #%d | Cliente: %-15s | Partition: %d | Offset: %d | Seq: %d | Pedido: %s | Produto: %s",
				contador,
				pedido.ClienteNome,
				partition,
				offset,
				pedido.SequenciaCliente,
				pedido.ID[:8]+"...",
				pedido.Produto)
		}

		contador++

		// Para após 50 mensagens para facilitar visualização
		if contador > 15000 {
			log.Println()
			log.Println("===========================================")
			log.Println("50 mensagens enviadas. Encerrando...")
			log.Println("===========================================")
			log.Println()
			log.Println("Resumo de mensagens por cliente:")
			for _, cliente := range clientes {
				log.Printf("  %s: %d mensagens", cliente.Nome, sequenciaCliente[cliente.ID])
			}
			log.Println()
			return
		}
	}
}

// Função auxiliar para variar os produtos
func getProdutoAleatorio() string {
	produtos := []string{
		"Notebook Dell XPS 15",
		"Mouse Logitech MX Master",
		"Teclado Mecânico Keychron",
		"Monitor LG 27 4K",
		"Webcam Logitech C920",
		"Headset Gamer HyperX",
		"SSD Samsung 1TB",
		"Memória RAM Corsair 16GB",
		"Placa de Vídeo RTX 4070",
		"Mousepad Gamer Extended",
		"Hub USB-C 7 portas",
		"Microfone Blue Yeti",
		"Cadeira Gamer DXRacer",
		"Mesa Digitalizadora Wacom",
		"Switch Gigabit 8 portas",
	}
	return produtos[rand.Intn(len(produtos))]
}
