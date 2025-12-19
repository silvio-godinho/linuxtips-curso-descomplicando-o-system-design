# SAGA Pattern - Orquestrado com Golang e Kafka

![SAGA Pattern](https://img.shields.io/badge/Pattern-SAGA-blue)
![Go Version](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)
![Kafka](https://img.shields.io/badge/Kafka-7.5-231F20?logo=apache-kafka)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-316192?logo=postgresql)

## 📋 Sobre o Projeto

Implementação completa do **padrão SAGA Orquestrado** em uma arquitetura de microsserviços utilizando:

- **Golang 1.23** para todos os serviços
- **Apache Kafka** para comunicação assíncrona (Command/Reply)
- **PostgreSQL** para persistência de eventos de domínio
- **Docker Compose** para orquestração da infraestrutura

## 🏗️ Arquitetura

```
┌─────────────────────────────────────────────────────────┐
│              ORQUESTRADOR SAGA                          │
│           (Máquina de Estados)                          │
│                                                         │
│  PENDING → ORDER_VALIDATED → STOCK_RESERVED →          │
│  PAYMENT_PROCESSED → DELIVERY_SCHEDULED → COMPLETED    │
└────────────┬────────────────────────┬───────────────────┘
             │   APACHE KAFKA         │
    ┌────────┴────────┐      ┌────────┴────────┐
    │   COMMANDS      │      │     REPLIES     │
    └─────────────────┘      └─────────────────┘
             │                        │
    ┌────────┴────────────────────────┴──────────┐
    │                                             │
┌───▼───┐  ┌────▼────┐  ┌────▼────┐  ┌────▼────┐
│Pedidos│  │ Estoque │  │Pagamen- │  │Entregas │
│Service│  │ Service │  │toService│  │ Service │
└───────┘  └─────────┘  └─────────┘  └─────────┘
```

## 🧩 Componentes

### Microsserviços

1. **Orquestrador SAGA** - Gerencia o fluxo da transação distribuída
2. **Serviço de Pedidos** - Valida e gerencia pedidos
3. **Serviço de Estoque** - Gerencia reservas de estoque
4. **Serviço de Pagamentos** - Processa pagamentos
5. **Serviço de Entregas** - Agenda entregas
6. **Simulador** - Aplicação para testes e simulações

### Infraestrutura

- **Kafka** (KRaft mode) - Message broker
- **Kafka UI** - Interface web para monitoramento
- **PostgreSQL** - 5 bancos de dados (um por serviço)

## 🚀 Quick Start

### 1. Subir toda a stack

```bash
docker-compose up -d
```

### 2. Verificar status

```bash
./scripts/check-status.sh
```

### 3. Executar simulador de testes

```bash
cd simulador
go run main.go
```

**Ou usando o script bash:**

```bash
./scripts/test-saga.sh
```

### 4. Acessar Kafka UI

```
http://localhost:8090
```

## 🧪 Testando com o Simulador

O simulador em Golang oferece um menu interativo:

```
╔════════════════════════════════════════════════╗
║  🧪 Simulador de Testes - SAGA Pattern        ║
║     Orquestrado com Golang e Kafka            ║
╚════════════════════════════════════════════════╝

Escolha uma opção:

1) 🎯 Enviar 1 pedido (alta chance de sucesso)
2) 🔥 Enviar 20 pedidos (para forçar falhas)
3) 🎲 Enviar N pedidos customizados
4) 👁️  Monitorar tópicos de reply
5) ❌ Sair
```

### Opções de Teste

**Opção 1**: Envia um único pedido para validar o fluxo completo

**Opção 2**: Envia 20 pedidos para demonstrar compensações
- ~2 pedidos falham no Estoque (10% chance)
- ~1 pedido falha no Pagamento (5% chance)

**Opção 3**: Permite enviar quantidade customizada

**Opção 4**: Monitora todos os tópicos de reply em tempo real

## 🔄 Fluxo da SAGA

### Fluxo de Sucesso

```
VALIDATE_ORDER → RESERVE_STOCK → PROCESS_PAYMENT → 
SCHEDULE_DELIVERY → COMPLETED ✅
```

### Fluxo com Compensação

```
VALIDATE_ORDER → RESERVE_STOCK → PROCESS_PAYMENT (FALHA) →
COMPENSATING → CANCEL_PAYMENT → RELEASE_STOCK → 
CANCEL_ORDER → FAILED ❌
```

## 📊 Monitoramento

### Logs dos Serviços

```bash
# Ver logs em tempo real
docker-compose logs -f

# Filtrar por serviço
docker-compose logs -f orquestrador
docker-compose logs -f pedidos
```

### Kafka UI

Acesse http://localhost:8090 para:
- Visualizar todos os tópicos
- Inspecionar mensagens
- Monitorar consumer groups

### Bancos de Dados

```bash
# Conectar ao banco do orquestrador
docker exec -it saga-db-orquestrador psql -U postgres -d orquestrador

# Ver eventos da SAGA
SELECT saga_id, state, created_at 
FROM saga_events 
ORDER BY created_at DESC 
LIMIT 10;
```

## 📂 Estrutura do Projeto

```
.
├── docker-compose.yml          # Orquestração completa
├── ARCHITECTURE.md             # Documentação detalhada
├── QUICKSTART.md               # Guia rápido
├── orquestrador/               # Serviço orquestrador
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── pedidos/                    # Serviço de pedidos
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── estoque/                    # Serviço de estoque
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── pagamentos/                 # Serviço de pagamentos
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── entregas/                   # Serviço de entregas
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── simulador/                  # Simulador de testes em Go
│   ├── main.go
│   ├── go.mod
│   ├── Dockerfile
│   └── README.md
└── scripts/                    # Scripts utilitários
    ├── test-saga.sh           # Teste via bash/kcat
    ├── check-status.sh        # Verificar status
    └── clean-all.sh           # Limpar tudo
```

## 🛠️ Comandos Úteis

### Docker Compose

```bash
# Subir serviços
docker-compose up -d

# Ver logs
docker-compose logs -f

# Parar serviços
docker-compose down

# Limpar tudo (volumes e imagens)
docker-compose down -v --rmi all
```

### Simulador

```bash
# Executar simulador
cd simulador && go run main.go

# Compilar
cd simulador && go build -o simulador

# Executar via Docker
docker build -t saga-simulador ./simulador
docker run -it --network saga_saga saga-simulador
```

### Kafka (com kcat)

```bash
# Listar tópicos
kcat -b localhost:9092 -L

# Consumir mensagens
kcat -b localhost:9092 -t pedidos-reply -C

# Produzir mensagem
echo '{"test": "message"}' | kcat -b localhost:9092 -t pedidos-commands -P
```

## 📈 Estados da SAGA

| Estado | Descrição |
|--------|-----------|
| `PENDING` | Estado inicial |
| `ORDER_VALIDATED` | Pedido validado com sucesso |
| `STOCK_RESERVED` | Estoque reservado |
| `PAYMENT_PROCESSED` | Pagamento processado |
| `DELIVERY_SCHEDULED` | Entrega agendada |
| `COMPLETED` | SAGA concluída com sucesso ✅ |
| `COMPENSATING` | Executando compensações |
| `FAILED` | SAGA falhou após compensações ❌ |

## 🎯 Características Implementadas

### ✅ Padrão SAGA Orquestrado
- Orquestrador centralizado
- Máquina de estados explícita
- Transações de longa duração

### ✅ Padrão Command/Reply
- Comandos assíncronos
- Respostas processadas
- Desacoplamento temporal

### ✅ Compensações Automáticas
- Ações reversas em caso de falha
- Ordem inversa de execução
- Consistência eventual

### ✅ Event Sourcing
- Todos os eventos persistidos
- Histórico completo
- Auditoria completa

### ✅ Resiliência
- Retry automático via Kafka
- Healthchecks em todos os serviços
- Restart policies

## 📚 Documentação Adicional

- [ARCHITECTURE.md](./ARCHITECTURE.md) - Arquitetura detalhada
- [QUICKSTART.md](./QUICKSTART.md) - Guia rápido
- [simulador/README.md](./simulador/README.md) - Documentação do simulador

## 🔧 Requisitos

- Docker >= 20.10
- Docker Compose >= 2.0
- Go 1.23+ (para desenvolvimento)
- 8GB RAM disponível
- Portas livres: 5432-5436, 8090, 9092-9093

## 🐛 Troubleshooting

### Serviços não iniciam

```bash
# Verificar logs
docker-compose logs

# Verificar status
./scripts/check-status.sh

# Reiniciar serviços
docker-compose restart
```

### Erro de conexão com Kafka

Aguarde o Kafka ficar saudável:
```bash
docker-compose logs -f kafka
```

### Limpar e recomeçar

```bash
./scripts/clean-all.sh
docker-compose up -d
```

## 🎓 Conceitos Demonstrados

- ✅ SAGA Pattern Orquestrado
- ✅ Event-Driven Architecture
- ✅ Compensating Transactions
- ✅ Command/Reply Pattern
- ✅ Event Sourcing (simplificado)
- ✅ Microsserviços com Golang
- ✅ Message Broker (Kafka)
- ✅ Docker Compose para orquestração

## 📖 Referências

- [Microservices Patterns - Chris Richardson](https://microservices.io/patterns/data/saga.html)
- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [SAGA Pattern - Microsoft](https://learn.microsoft.com/en-us/azure/architecture/reference-architectures/saga/saga)

## 👥 Autor

Projeto desenvolvido para fins didáticos como parte do curso **Descomplicando o System Design** da **LINUXtips**.

---

**🎯 Desenvolvido com foco em arquitetura, design de sistemas distribuídos e boas práticas de engenharia de software.**
