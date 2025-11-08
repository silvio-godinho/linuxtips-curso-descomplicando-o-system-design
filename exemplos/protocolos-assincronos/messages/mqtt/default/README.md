# MQTT Demo - LinuxTips

Exemplo de implementação de MQTT com produtor e consumidor usando Go.

## 🚀 Como executar

### Pré-requisitos
- Docker
- Docker Compose

### Subir a infraestrutura completa

```bash
docker-compose up --build
```

Isso irá:
1. Subir o broker MQTT (Mosquitto) na porta 1883
2. Subir o produtor que publica mensagens a cada 5 segundos
3. Subir o consumidor que recebe as mensagens

### Ver os logs

```bash
# Ver todos os logs
docker-compose logs -f

# Ver apenas do producer
docker-compose logs -f producer

# Ver apenas do consumer
docker-compose logs -f consumer

# Ver apenas do broker
docker-compose logs -f mosquitto
```

### Parar a infraestrutura

```bash
docker-compose down
```

## 📦 Componentes

### Mosquitto (Broker MQTT)
- Porta: 1883 (MQTT)
- Porta: 9001 (WebSocket)
- Configurado para aceitar conexões anônimas

### Producer
- Publica mensagens no tópico `linuxtips/demo`
- Intervalo: 5 segundos
- QoS: 1 (At least once)

### Consumer
- Subscreve no tópico `linuxtips/demo`
- QoS: 1 (At least once)
- Reconexão automática

## 🔧 Executar localmente (sem Docker)

### Instalar dependências

```bash
go mod download
```

### Executar o consumer

```bash
go run consumer.go
```

### Executar o producer (em outro terminal)

```bash
go run producer.go
```

## 📚 Conceitos MQTT

### QoS (Quality of Service)
- **QoS 0**: At most once (Entrega não garantida)
- **QoS 1**: At least once (Entrega garantida, pode duplicar)
- **QoS 2**: Exactly once (Entrega garantida uma única vez)

### Tópicos
O exemplo usa o tópico `linuxtips/demo`, mas você pode customizar via variável de ambiente:

```bash
export MQTT_TOPIC="seu/topico"
```

### Clean Session
- `true`: O broker não armazena mensagens offline
- `false`: O broker mantém mensagens para clientes desconectados
