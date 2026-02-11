# WB L0 

## Установка и запуск

Перед запуском создайте файл конфигурации:

```bash
cp .env.example .env
```

### Быстрый старт

```bash
make up
```

### Ручной запуск

```bash
  #Запуск redis, kafka, postgresql и jaeger
  make up-infra
  # Запуск скрипта имитирующий приход заказов
  make prod-run
  # Запуск сервера, если по каким-то причинам миграции не применились, стоит использовать
  # make migrate-up
  make run
```

## Пример запроса 

Перейти в [веб-интерфейс](http://localhost:8080) и вставить в форму любой `id`,
например `b563feb7b2b84b6test` либо `1`, после чего получить в ответ валидный json если он сохранен в кэше/базе,
если же нет, получить ошибку 404.

### Пример ответа 
```json
{
  "order_uid": "b563feb7b2b84b6test",
  "track_number": "WBILMTESTTRACK",
  "entry": "WBIL",
  "delivery": {
    "name": "Test Testov",
    "phone": "+9720000000",
    "zip": "2639809",
    "city": "Kiryat Mozkin",
    "address": "Ploshad Mira 15",
    "region": "Kraiot",
    "email": "test@gmail.com"
  },
  "payment": {
    "transaction": "b563feb7b2b84b6test",
    "request_id": "",
    "currency": "USD",
    "provider": "wbpay",
    "amount": 1817,
    "payment_dt": 1637907727,
    "bank": "alpha",
    "delivery_cost": 1500,
    "goods_total": 317,
    "custom_fee": 0
  },
  "items": [
    {
      "chrt_id": 9934930,
      "track_number": "WBILMTESTTRACK",
      "price": 453,
      "rid": "ab4219087a764ae0btest",
      "name": "Mascaras",
      "sale": 30,
      "size": "0",
      "total_price": 317,
      "nm_id": 2389212,
      "brand": "Vivienne Sabo",
      "status": 202
    }
  ],
  "locale": "en",
  "internal_signature": "",
  "customer_id": "test",
  "delivery_service": "meest",
  "shardkey": "9",
  "sm_id": 99,
  "date_created": "2026-02-09T18:54:42.205333612Z",
  "oof_shard": "1"
}
```

## Архитектура проекта

```
.
├── cmd
│   ├── app
│   └── producer
├── db
│   └── migrations
├── internal
│   ├── config
│   ├── lib
│   │   ├── log
│   │   ├── tracing
│   │   └── validator
│   ├── model
│   ├── repository
│   │   ├── postgresql
│   │   └── redis
│   ├── service
│   ├── storage
│   │   └── postgre
│   └── transport
│       ├── handlers
│       └── kafka
└── web
    └── static
```


## Jeager UI 

Для доступа к просмотрам трейсов стоит перейти по `http://localhost:16686`