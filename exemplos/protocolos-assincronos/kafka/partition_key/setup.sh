#!/bin/bash

echo "=========================================="
echo "Configurando tópico com partition key"
echo "=========================================="
echo ""

# Verifica se o Kafka está rodando
if ! docker ps | grep -q kafka; then
    echo "❌ Kafka não está rodando!"
    echo "Execute: cd .. && docker-compose up -d"
    exit 1
fi

echo "✅ Kafka está rodando"
echo ""

# Deleta o tópico se já existir
echo "🗑️  Deletando tópico antigo (se existir)..."
docker exec kafka kafka-topics --delete \
  --topic pedidos-particionados \
  --bootstrap-server localhost:9092 2>/dev/null || true

sleep 2

# Cria o tópico com 3 partições
echo ""
echo "📝 Criando tópico 'pedidos-particionados' com 3 partições..."
docker exec kafka kafka-topics --create \
  --topic pedidos-particionados \
  --bootstrap-server localhost:9092 \
  --partitions 3 \
  --replication-factor 1

echo ""
echo "📊 Detalhes do tópico:"
docker exec kafka kafka-topics --describe \
  --topic pedidos-particionados \
  --bootstrap-server localhost:9092

echo ""
echo "=========================================="
echo "✅ Setup completo!"
echo "=========================================="
echo ""
echo "Execute:"
echo "  go run main.go       # Producer"
echo "  go run consumer.go   # Consumer"
echo ""
