package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	_ "github.com/lib/pq"
)

// Command representa um comando recebido do orquestrador
type Command struct {
	CommandID   string                 `json:"command_id"`
	SagaID      string                 `json:"saga_id"`
	OrderID     string                 `json:"order_id"`
	CommandType string                 `json:"command_type"`
	Payload     map[string]interface{} `json:"payload"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Reply representa uma resposta para o orquestrador
type Reply struct {
	ReplyID   string                 `json:"reply_id"`
	CommandID string                 `json:"command_id"`
	SagaID    string                 `json:"saga_id"`
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// StockReservation representa uma reserva de estoque
type StockReservation struct {
	ID        string    `json:"id"`
	SagaID    string    `json:"saga_id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// StockService gerencia o estoque
type StockService struct {
	db       *sql.DB
	producer sarama.SyncProducer
	consumer sarama.ConsumerGroup
}

func main() {
	log.Println("Iniciando Serviço de Estoque...")

	// Conectar ao banco de dados
	db, err := connectDB()
	if err != nil {
		log.Fatal("Erro ao conectar no banco:", err)
	}
	defer db.Close()

	// Inicializar schema
	if err := initSchema(db); err != nil {
		log.Fatal("Erro ao inicializar schema:", err)
	}

	// Configurar Kafka Producer
	producer, err := setupProducer()
	if err != nil {
		log.Fatal("Erro ao configurar producer:", err)
	}
	defer producer.Close()

	// Configurar Kafka Consumer
	consumer, err := setupConsumer()
	if err != nil {
		log.Fatal("Erro ao configurar consumer:", err)
	}
	defer consumer.Close()

	service := &StockService{
		db:       db,
		producer: producer,
		consumer: consumer,
	}

	// Iniciar consumo de comandos
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go service.consumeCommands(ctx)

	// Aguardar sinal de término
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	<-sigterm

	log.Println("Encerrando Serviço de Estoque...")
}

func connectDB() (*sql.DB, error) {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "estoque")

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	// Tentar conectar com retry
	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			log.Println("Conectado ao banco de dados")
			return db, nil
		}
		log.Printf("⏳ Aguardando banco de dados... (%d/30)", i+1)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("timeout ao conectar no banco")
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS stock_reservations (
		id VARCHAR(100) PRIMARY KEY,
		saga_id VARCHAR(100) NOT NULL,
		product_id VARCHAR(100) NOT NULL,
		quantity INTEGER NOT NULL,
		status VARCHAR(50) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_saga_id ON stock_reservations(saga_id);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	log.Println("Schema do banco inicializado")
	return nil
}

func setupProducer() (sarama.SyncProducer, error) {
	brokers := []string{getEnv("KAFKA_BROKERS", "localhost:9092")}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	log.Println("Kafka Producer configurado")
	return producer, nil
}

func setupConsumer() (sarama.ConsumerGroup, error) {
	brokers := []string{getEnv("KAFKA_BROKERS", "localhost:9092")}

	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	consumer, err := sarama.NewConsumerGroup(brokers, "estoque-group", config)
	if err != nil {
		return nil, err
	}

	log.Println("Kafka Consumer configurado")
	return consumer, nil
}

// consumeCommands consome comandos do orquestrador
func (s *StockService) consumeCommands(ctx context.Context) {
	topics := []string{"estoque-commands"}
	handler := &ConsumerHandler{service: s}

	for {
		if err := s.consumer.Consume(ctx, topics, handler); err != nil {
			log.Printf("Erro ao consumir mensagens: %v", err)
		}

		if ctx.Err() != nil {
			return
		}
	}
}

// ConsumerHandler implementa sarama.ConsumerGroupHandler
type ConsumerHandler struct {
	service *StockService
}

func (h *ConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *ConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *ConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var cmd Command
		if err := json.Unmarshal(message.Value, &cmd); err != nil {
			log.Printf("Erro ao deserializar comando: %v", err)
			session.MarkMessage(message, "")
			continue
		}

		log.Printf("Comando recebido: %s (SAGA: %s)", cmd.CommandType, cmd.SagaID)

		// Processar comando
		reply := h.service.processCommand(&cmd)

		// Enviar resposta
		if err := h.service.sendReply(reply); err != nil {
			log.Printf("❌ Erro ao enviar reply: %v", err)
		}

		session.MarkMessage(message, "")
	}
	return nil
}

// processCommand processa um comando e retorna uma resposta
func (s *StockService) processCommand(cmd *Command) *Reply {
	reply := &Reply{
		ReplyID:   generateID(),
		CommandID: cmd.CommandID,
		SagaID:    cmd.SagaID,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
	}
	
	// Copiar payload para Data se existir
	if cmd.Payload != nil {
		for k, v := range cmd.Payload {
			reply.Data[k] = v
		}
	}

	switch cmd.CommandType {
	case "RESERVE_STOCK":
		// Reservar estoque (mockado com chance de falha)
		reservation := s.reserveStock(cmd)
		if reservation != nil {
			reply.Success = true
			reply.Message = "Estoque reservado com sucesso"
			reply.Data["reservation_id"] = reservation.ID
			log.Printf("Estoque reservado: %d unidades do produto %s",
				reservation.Quantity, reservation.ProductID)
		} else {
			reply.Success = false
			reply.Message = "Estoque insuficiente"
			log.Printf("Estoque insuficiente")
		}

	case "RELEASE_STOCK":
		// Liberar estoque (compensação)
		if err := s.releaseStock(cmd.SagaID); err != nil {
			reply.Success = false
			reply.Message = fmt.Sprintf("Erro ao liberar estoque: %v", err)
			log.Printf("❌ Erro ao liberar estoque: %v", err)
		} else {
			reply.Success = true
			reply.Message = "Estoque liberado com sucesso"
			log.Printf("Estoque liberado (SAGA: %s)", cmd.SagaID)
		}

	default:
		reply.Success = false
		reply.Message = fmt.Sprintf("Comando desconhecido: %s", cmd.CommandType)
		log.Printf("Comando desconhecido: %s", cmd.CommandType)
	}

	return reply
}

// reserveStock reserva estoque (mockado)
func (s *StockService) reserveStock(cmd *Command) *StockReservation {
	// Simulação de verificação de estoque
	// 10% de chance de falha para demonstrar compensação
	if rand.Intn(100) < 10 {
		log.Println("Simulando falha de estoque insuficiente")
		return nil
	}

	reservation := &StockReservation{
		ID:        generateID(),
		SagaID:    cmd.SagaID,
		ProductID: getStringFromPayload(cmd.Payload, "product_id", "PROD-001"),
		Quantity:  getIntFromPayload(cmd.Payload, "quantity", 1),
		Status:    "RESERVED",
		CreatedAt: time.Now(),
	}

	// Persistir no banco
	_, err := s.db.Exec(
		`INSERT INTO stock_reservations (id, saga_id, product_id, quantity, status)
		 VALUES ($1, $2, $3, $4, $5)`,
		reservation.ID, reservation.SagaID, reservation.ProductID,
		reservation.Quantity, reservation.Status,
	)

	if err != nil {
		log.Printf("❌ Erro ao salvar reserva: %v", err)
		return nil
	}

	return reservation
}

// releaseStock libera estoque
func (s *StockService) releaseStock(sagaID string) error {
	_, err := s.db.Exec(
		"UPDATE stock_reservations SET status = 'RELEASED' WHERE saga_id = $1",
		sagaID,
	)
	return err
}

// sendReply envia uma resposta para o orquestrador
func (s *StockService) sendReply(reply *Reply) error {
	data, err := json.Marshal(reply)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: "estoque-reply",
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = s.producer.SendMessage(msg)
	if err != nil {
		return err
	}

	log.Printf("Reply enviado: Success=%t, Message=%s", reply.Success, reply.Message)
	return nil
}

// Funções auxiliares
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getStringFromPayload(payload map[string]interface{}, key, defaultValue string) string {
	if val, ok := payload[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return defaultValue
}

func getIntFromPayload(payload map[string]interface{}, key string, defaultValue int) int {
	if val, ok := payload[key]; ok {
		if intVal, ok := val.(float64); ok {
			return int(intVal)
		}
	}
	return defaultValue
}
