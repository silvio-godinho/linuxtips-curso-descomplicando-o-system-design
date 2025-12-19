package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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

// Delivery representa uma entrega
type Delivery struct {
	ID             string    `json:"id"`
	SagaID         string    `json:"saga_id"`
	OrderID        string    `json:"order_id"`
	Address        string    `json:"address"`
	ScheduledDate  time.Time `json:"scheduled_date"`
	Status         string    `json:"status"`
	TrackingNumber string    `json:"tracking_number"`
	CreatedAt      time.Time `json:"created_at"`
}

// DeliveryService gerencia entregas
type DeliveryService struct {
	db       *sql.DB
	producer sarama.SyncProducer
	consumer sarama.ConsumerGroup
}

func main() {
	log.Println("Iniciando Serviço de Entregas...")

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

	service := &DeliveryService{
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

	log.Println("Encerrando Serviço de Entregas...")
}

func connectDB() (*sql.DB, error) {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "entregas")

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
	CREATE TABLE IF NOT EXISTS deliveries (
		id VARCHAR(100) PRIMARY KEY,
		saga_id VARCHAR(100) NOT NULL,
		order_id VARCHAR(100) NOT NULL,
		address TEXT NOT NULL,
		scheduled_date TIMESTAMP NOT NULL,
		status VARCHAR(50) NOT NULL,
		tracking_number VARCHAR(100),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_saga_id ON deliveries(saga_id);
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

	consumer, err := sarama.NewConsumerGroup(brokers, "entregas-group", config)
	if err != nil {
		return nil, err
	}

	log.Println("Kafka Consumer configurado")
	return consumer, nil
}

// consumeCommands consome comandos do orquestrador
func (s *DeliveryService) consumeCommands(ctx context.Context) {
	topics := []string{"entregas-commands"}
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
	service *DeliveryService
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
func (s *DeliveryService) processCommand(cmd *Command) *Reply {
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
	case "SCHEDULE_DELIVERY":
		// Agendar entrega (mockado)
		delivery := s.scheduleDelivery(cmd)
		if delivery != nil {
			reply.Success = true
			reply.Message = "Entrega agendada com sucesso"
			reply.Data["delivery_id"] = delivery.ID
			reply.Data["tracking_number"] = delivery.TrackingNumber
			reply.Data["scheduled_date"] = delivery.ScheduledDate.Format(time.RFC3339)
			log.Printf("Entrega agendada: %s (Tracking: %s)",
				delivery.ScheduledDate.Format("02/01/2006"), delivery.TrackingNumber)
		} else {
			reply.Success = false
			reply.Message = "Falha ao agendar entrega"
			log.Printf("Falha ao agendar entrega")
		}

	case "CANCEL_DELIVERY":
		// Cancelar entrega (compensação)
		if err := s.cancelDelivery(cmd.SagaID); err != nil {
			reply.Success = false
			reply.Message = fmt.Sprintf("Erro ao cancelar entrega: %v", err)
			log.Printf("❌ Erro ao cancelar entrega: %v", err)
		} else {
			reply.Success = true
			reply.Message = "Entrega cancelada com sucesso"
			log.Printf("Entrega cancelada (SAGA: %s)", cmd.SagaID)
		}

	default:
		reply.Success = false
		reply.Message = fmt.Sprintf("Comando desconhecido: %s", cmd.CommandType)
		log.Printf("Comando desconhecido: %s", cmd.CommandType)
	}

	return reply
}

// scheduleDelivery agenda uma entrega (mockado)
func (s *DeliveryService) scheduleDelivery(cmd *Command) *Delivery {
	// Simulação de agendamento de entrega
	// Sempre sucede - última etapa da SAGA

	scheduledDate := time.Now().Add(48 * time.Hour) // 2 dias a partir de agora

	delivery := &Delivery{
		ID:             generateID(),
		SagaID:         cmd.SagaID,
		OrderID:        getStringFromPayload(cmd.Payload, "order_id", ""),
		Address:        getStringFromPayload(cmd.Payload, "address", "Rua Exemplo, 123"),
		ScheduledDate:  scheduledDate,
		Status:         "SCHEDULED",
		TrackingNumber: fmt.Sprintf("TRK-%d", time.Now().Unix()),
		CreatedAt:      time.Now(),
	}

	// Persistir no banco
	_, err := s.db.Exec(
		`INSERT INTO deliveries (id, saga_id, order_id, address, scheduled_date, status, tracking_number)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		delivery.ID, delivery.SagaID, delivery.OrderID, delivery.Address,
		delivery.ScheduledDate, delivery.Status, delivery.TrackingNumber,
	)

	if err != nil {
		log.Printf("❌ Erro ao salvar entrega: %v", err)
		return nil
	}

	return delivery
}

// cancelDelivery cancela uma entrega
func (s *DeliveryService) cancelDelivery(sagaID string) error {
	_, err := s.db.Exec(
		"UPDATE deliveries SET status = 'CANCELLED' WHERE saga_id = $1",
		sagaID,
	)
	return err
}

// sendReply envia uma resposta para o orquestrador
func (s *DeliveryService) sendReply(reply *Reply) error {
	data, err := json.Marshal(reply)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: "entregas-reply",
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
