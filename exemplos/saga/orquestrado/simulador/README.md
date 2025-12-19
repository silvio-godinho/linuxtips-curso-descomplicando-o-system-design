# Simulador de Testes SAGA

Aplicação em Golang para simular e testar o fluxo da SAGA enviando pedidos para o Kafka.

## 🚀 Como Usar

### Opção 1: Executar localmente

```bash
cd simulador
go run main.go
```

### Opção 2: Compilar e executar

```bash
cd simulador
go build -o simulador
./simulador
```

### Opção 3: Via Docker

```bash
# Construir imagem
docker build -t saga-simulador ./simulador

# Executar
docker run -it --network saga-network saga-simulador
```

## 📋 Funcionalidades

### 1. Enviar 1 pedido
Envia um único pedido para teste. Alta chance de sucesso.

### 2. Enviar 20 pedidos
Envia múltiplos pedidos para forçar falhas e compensações.
- ~2 pedidos falham no Estoque (10% chance)
- ~1 pedido falha no Pagamento (5% chance)

### 3. Enviar N pedidos customizados
Permite especificar quantos pedidos enviar.

### 4. Monitorar tópicos de reply
Inicia um consumer que monitora todos os tópicos de resposta em tempo real.

## 🎨 Output Colorido

O simulador usa cores ANSI para facilitar a visualização:
- 🟢 Verde: Sucesso
- 🔴 Vermelho: Erro
- 🟡 Amarelo: Aviso
- 🔵 Azul: Informação
- 🟣 Roxo: IDs importantes
- 🔷 Ciano: Comandos e URLs

## 📊 Exemplo de Uso

```
╔════════════════════════════════════════════════╗
║  🧪 Simulador de Testes - SAGA Pattern        ║
║     Orquestrado com Golang e Kafka            ║
╚════════════════════════════════════════════════╝

✅ Kafka Producer configurado

Escolha uma opção:

1) 🎯 Enviar 1 pedido (alta chance de sucesso)
2) 🔥 Enviar 20 pedidos (para forçar falhas)
3) 🎲 Enviar N pedidos customizados
4) 👁️  Monitorar tópicos de reply
5) ❌ Sair

Opção: 1

🚀 Enviando pedido único...
SAGA ID: saga-1734516789-1234

✅ Pedido enviado com sucesso!

📊 Para acompanhar o processamento:
   docker-compose logs -f orquestrador
   docker-compose logs -f pedidos

🌐 Ou acesse o Kafka UI:
   http://localhost:8090
```

## 🔧 Configuração

O simulador se conecta ao Kafka via variável de ambiente:

```bash
export KAFKA_BROKERS=localhost:9092
```

Ou usa o padrão `localhost:9092` se não configurado.

## 🐛 Troubleshooting

### Erro de conexão com Kafka

```
❌ Erro ao configurar Kafka producer: ...
```

**Solução**: Verifique se o Kafka está rodando:
```bash
docker-compose ps kafka
```

### Tópicos não encontrados

```
⚠️  Aviso: Tópico pedidos-reply não encontrado
```

**Solução**: Os tópicos são criados automaticamente. Envie um pedido primeiro antes de monitorar.
