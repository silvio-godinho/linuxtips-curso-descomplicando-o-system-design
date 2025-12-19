package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
)

// Cores ANSI para output colorido
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

// Command representa um comando enviado para o Kafka
type Command struct {
	CommandID   string                 `json:"command_id"`
	SagaID      string                 `json:"saga_id"`
	OrderID     string                 `json:"order_id"`
	CommandType string                 `json:"command_type"`
	Payload     map[string]interface{} `json:"payload"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Reply representa uma resposta recebida do Kafka
type Reply struct {
	ReplyID   string                 `json:"reply_id"`
	CommandID string                 `json:"command_id"`
	SagaID    string                 `json:"saga_id"`
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// Simulator gerencia a simulação de testes da SAGA
type Simulator struct {
	producer sarama.SyncProducer
	brokers  []string
}

func main() {
	printHeader()

	brokers := []string{getEnv("KAFKA_BROKERS", "localhost:9092")}

	sim := &Simulator{
		brokers: brokers,
	}

	// Configurar producer
	if err := sim.setupProducer(); err != nil {
		log.Fatalf("%sErro ao configurar Kafka producer: %v%s\n", ColorRed, err, ColorReset)
	}
	defer sim.producer.Close()

	// Menu principal
	sim.showMenu()
}

func printHeader() {
	fmt.Println()
	fmt.Printf("%s╔════════════════════════════════════════════════╗%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s║  Simulador de Testes - SAGA Pattern           ║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s║     Orquestrado com Golang e Kafka            ║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s╚════════════════════════════════════════════════╝%s\n", ColorCyan, ColorReset)
	fmt.Println()
}

func (s *Simulator) setupProducer() error {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	producer, err := sarama.NewSyncProducer(s.brokers, config)
	if err != nil {
		return err
	}

	s.producer = producer
	fmt.Printf("%sKafka Producer configurado%s\n\n", ColorGreen, ColorReset)
	return nil
}

func (s *Simulator) showMenu() {
	for {
		fmt.Printf("%sEscolha uma opção:%s\n", ColorYellow, ColorReset)
		fmt.Println()
		fmt.Println("1) Enviar 1 pedido (alta chance de sucesso)")
		fmt.Println("2) Enviar 20 pedidos (para forçar falhas)")
		fmt.Println("3) Enviar N pedidos customizados")
		fmt.Println("4) Monitorar tópicos de reply")
		fmt.Println("5) Sair")
		fmt.Println()
		fmt.Print("Opção: ")

		var option int
		fmt.Scanln(&option)
		fmt.Println()

		switch option {
		case 1:
			s.sendSingleOrder()
		case 2:
			s.sendMultipleOrders(20)
		case 3:
			s.sendCustomOrders()
		case 4:
			s.monitorReplies()
		case 5:
			fmt.Printf("%sEncerrando simulador...%s\n", ColorGreen, ColorReset)
			return
		default:
			fmt.Printf("%sOpção inválida%s\n\n", ColorRed, ColorReset)
		}
	}
}

func (s *Simulator) sendSingleOrder() {
	orderID := generateID()

	fmt.Printf("%sEnviando pedido único...%s\n", ColorBlue, ColorReset)
	fmt.Printf("Order ID: %s%s%s\n\n", ColorPurple, orderID, ColorReset)

	orderData := map[string]interface{}{
		"order_id":     orderID,
		"customer_id":  "CUST-001",
		"product_id":   "PROD-001",
		"quantity":     1,
		"total_amount": 299.99,
		"address":      "Rua Exemplo, 123 - São Paulo/SP",
	}

	if err := s.sendOrderToProcess(orderData); err != nil {
		fmt.Printf("%sErro ao enviar pedido: %v%s\n\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%sPedido enviado com sucesso!%s\n", ColorGreen, ColorReset)
	fmt.Println()
	fmt.Println("Para acompanhar o processamento:")
	fmt.Printf("   %sdocker-compose logs -f orquestrador%s\n", ColorCyan, ColorReset)
	fmt.Printf("   %sdocker-compose logs -f pedidos%s\n", ColorCyan, ColorReset)
	fmt.Println()
	fmt.Println("Ou acesse o Kafka UI:")
	fmt.Printf("   %shttp://localhost:8090%s\n", ColorCyan, ColorReset)
	fmt.Println()
}

func (s *Simulator) sendMultipleOrders(count int) {
	fmt.Printf("%sEnviando %d pedidos para forçar falhas...%s\n\n", ColorYellow, count, ColorReset)

	successCount := 0

	for i := 1; i <= count; i++ {
		orderID := generateID()

		customerID := fmt.Sprintf("CUST-%03d", (i%10)+1)
		productID := fmt.Sprintf("PROD-%03d", (i%5)+1)
		quantity := (i % 5) + 1
		amount := float64(quantity) * (99.99 + float64(i%20)*10)

		orderData := map[string]interface{}{
			"order_id":     orderID,
			"customer_id":  customerID,
			"product_id":   productID,
			"quantity":     quantity,
			"total_amount": amount,
			"address":      fmt.Sprintf("Rua %d, São Paulo/SP", i),
		}

		if err := s.sendOrderToProcess(orderData); err != nil {
			fmt.Printf("%sErro no pedido %d: %v%s\n", ColorRed, i, err, ColorReset)
		} else {
			successCount++
			if i%5 == 0 {
				fmt.Printf("  %d/%d pedidos enviados...\n", i, count)
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println()
	fmt.Printf("%s%d/%d pedidos enviados com sucesso!%s\n", ColorGreen, successCount, count, ColorReset)
	fmt.Println()
	fmt.Printf("%sDica: Com %d pedidos, estatisticamente:%s\n", ColorYellow, count, ColorReset)
	fmt.Printf("   - ~2 pedidos devem falhar no Estoque (10%% chance)\n")
	fmt.Printf("   - ~1 pedido deve falhar no Pagamento (5%% chance)\n")
	fmt.Printf("   - O restante deve ser completado com sucesso\n")
	fmt.Println()
	fmt.Println("Monitore os logs para ver compensações:")
	fmt.Printf("   %sdocker-compose logs -f orquestrador | grep -i compensat%s\n", ColorCyan, ColorReset)
	fmt.Println()
}

func (s *Simulator) sendCustomOrders() {
	var count int
	fmt.Print("Quantos pedidos deseja enviar? ")
	fmt.Scanln(&count)

	if count <= 0 {
		fmt.Printf("%sQuantidade inválida%s\n\n", ColorRed, ColorReset)
		return
	}

	if count > 100 {
		fmt.Printf("%sAtencao: Enviar muitos pedidos pode sobrecarregar o sistema%s\n", ColorYellow, ColorReset)
		fmt.Print("Continuar? (s/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "s" && confirm != "S" {
			fmt.Println("Cancelado.")
			return
		}
	}

	fmt.Println()
	s.sendMultipleOrders(count)
}

func (s *Simulator) monitorReplies() {
	fmt.Printf("%sModo de Monitoramento%s\n", ColorCyan, ColorReset)
	fmt.Println()
	fmt.Println("Iniciando consumidor para monitorar replies...")
	fmt.Println("Pressione Ctrl+C para sair")
	fmt.Println()

	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumer(s.brokers, config)
	if err != nil {
		fmt.Printf("%sErro ao criar consumer: %v%s\n\n", ColorRed, err, ColorReset)
		return
	}
	defer consumer.Close()

	topics := []string{
		"pedido-saga-pedido-processado", // Tópico de conclusão da SAGA
		"pedidos-reply",
		"estoque-reply",
		"pagamentos-reply",
		"entregas-reply",
	}

	// Criar canais para cada tópico
	for _, topic := range topics {
		partitions, err := consumer.Partitions(topic)
		if err != nil {
			fmt.Printf("%sAviso: Tópico %s não encontrado%s\n", ColorYellow, topic, ColorReset)
			continue
		}

		for _, partition := range partitions {
			pc, err := consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
			if err != nil {
				fmt.Printf("%sErro ao consumir %s: %v%s\n", ColorRed, topic, err, ColorReset)
				continue
			}

			go func(topic string, pc sarama.PartitionConsumer) {
				defer pc.Close()

				for msg := range pc.Messages() {
					var reply Reply
					if err := json.Unmarshal(msg.Value, &reply); err != nil {
						continue
					}

					color := ColorGreen
					status := "SUCCESS"
					if !reply.Success {
						color = ColorRed
						status = "FAILED"
					}

					fmt.Printf("%s[%s] %s %s - SAGA: %s - %s%s\n",
						color, topic, status, reply.Message, reply.SagaID, time.Now().Format("15:04:05"), ColorReset)
				}
			}(topic, pc)
		}
	}

	fmt.Printf("%sMonitoramento ativo em todos os tópicos%s\n\n", ColorGreen, ColorReset)

	// Aguardar indefinidamente
	select {}
}

// sendOrderToProcess publica pedido no tópico de início da SAGA
func (s *Simulator) sendOrderToProcess(orderData map[string]interface{}) error {
	data, err := json.Marshal(orderData)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: "pedido-saga-pedido-processar",
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = s.producer.SendMessage(msg)
	return err
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
