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

// Payment representa um pagamento
type Payment struct {
	ID            string    `json:"id"`
	SagaID        string    `json:"saga_id"`
	OrderID       string    `json:"order_id"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	TransactionID string    `json:"transaction_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// PaymentService gerencia pagamentos
type PaymentService struct {
	db       *sql.DB
	producer sarama.SyncProducer
	consumer sarama.ConsumerGroup
}

func main() {
	log.Println("Iniciando Serviço de Pagamentos...")

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

	service := &PaymentService{
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

	log.Println("Encerrando Serviço de Pagamentos...")
}

func connectDB() (*sql.DB, error) {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "pagamentos")

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
		log.Printf("Aguardando banco de dados... (%d/30)", i+1)
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("timeout ao conectar no banco")
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS payments (
		id VARCHAR(100) PRIMARY KEY,
		saga_id VARCHAR(100) NOT NULL,
		order_id VARCHAR(100) NOT NULL,
		amount DECIMAL(10,2) NOT NULL,
		status VARCHAR(50) NOT NULL,
		transaction_id VARCHAR(100),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_saga_id ON payments(saga_id);
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

	consumer, err := sarama.NewConsumerGroup(brokers, "pagamentos-group", config)
	if err != nil {
		return nil, err
	}

	log.Println("Kafka Consumer configurado")
	return consumer, nil
}

// consumeCommands consome comandos do orquestrador
func (s *PaymentService) consumeCommands(ctx context.Context) {
	topics := []string{"pagamentos-commands"}
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
	service *PaymentService
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
			log.Printf("Erro ao enviar reply: %v", err)
		}

		session.MarkMessage(message, "")
	}
	return nil
}

// processCommand processa um comando e retorna uma resposta
func (s *PaymentService) processCommand(cmd *Command) *Reply {
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
	case "PROCESS_PAYMENT":
		// Processar pagamento (mockado com chance de falha)
		payment := s.processPayment(cmd)
		if payment != nil {
			reply.Success = true
			reply.Message = "Pagamento processado com sucesso"
			reply.Data["payment_id"] = payment.ID
			reply.Data["transaction_id"] = payment.TransactionID
			log.Printf("Pagamento processado: R$ %.2f (Transaction: %s)",
				payment.Amount, payment.TransactionID)
		} else {
			reply.Success = false
			reply.Message = "Falha no processamento do pagamento"
			log.Printf("Falha no processamento do pagamento")
		}

	case "CANCEL_PAYMENT":
		// Cancelar pagamento (compensação)
		if err := s.cancelPayment(cmd.SagaID); err != nil {
			reply.Success = false
			reply.Message = fmt.Sprintf("Erro ao cancelar pagamento: %v", err)
			log.Printf("❌ Erro ao cancelar pagamento: %v", err)
		} else {
			reply.Success = true
			reply.Message = "Pagamento cancelado com sucesso"
			log.Printf("Pagamento cancelado (SAGA: %s)", cmd.SagaID)
		}

	default:
		reply.Success = false
		reply.Message = fmt.Sprintf("Comando desconhecido: %s", cmd.CommandType)
		log.Printf("Comando desconhecido: %s", cmd.CommandType)
	}

	return reply
}

// processPayment processa um pagamento (mockado)
func (s *PaymentService) processPayment(cmd *Command) *Payment {
	// Simulação de processamento de pagamento
	// 5% de chance de falha para demonstrar compensação
	if rand.Intn(100) < 5 {
		log.Println("Simulando falha no gateway de pagamento")
		return nil
	}

	payment := &Payment{
		ID:            generateID(),
		SagaID:        cmd.SagaID,
		OrderID:       getStringFromPayload(cmd.Payload, "order_id", ""),
		Amount:        getFloatFromPayload(cmd.Payload, "total_amount", 0.0),
		Status:        "APPROVED",
		TransactionID: fmt.Sprintf("TXN-%d", time.Now().Unix()),
		CreatedAt:     time.Now(),
	}

	// Persistir no banco
	_, err := s.db.Exec(
		`INSERT INTO payments (id, saga_id, order_id, amount, status, transaction_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		payment.ID, payment.SagaID, payment.OrderID,
		payment.Amount, payment.Status, payment.TransactionID,
	)

	if err != nil {
		log.Printf("❌ Erro ao salvar pagamento: %v", err)
		return nil
	}

	return payment
}

// cancelPayment cancela um pagamento
func (s *PaymentService) cancelPayment(sagaID string) error {
	_, err := s.db.Exec(
		"UPDATE payments SET status = 'CANCELLED' WHERE saga_id = $1",
		sagaID,
	)
	return err
}

// sendReply envia uma resposta para o orquestrador
func (s *PaymentService) sendReply(reply *Reply) error {
	data, err := json.Marshal(reply)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: "pagamentos-reply",
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

func getFloatFromPayload(payload map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := payload[key]; ok {
		if floatVal, ok := val.(float64); ok {
			return floatVal
		}
	}
	return defaultValue
}
