# PricePulse

PricePulse ежедневно фиксирует цены товаров на маркетплейсах. Источники подключаются как независимые Go-адаптеры с единым контрактом.

## Быстрый старт

```sh
make run-api     # API на http://localhost:8080
make run-worker  # один проход фонового сборщика
make test
```

## Docker

```sh
docker compose up --build api
docker compose --profile worker run --rm worker
```

`Dockerfile` собирает статические Go-бинарники в Alpine и помещает их в отдельные минимальные `scratch`-образы: `api` и `worker`. В них также есть CA-сертификаты для HTTPS-запросов будущих адаптеров. В GitHub Actions workflow [ci.yml](.github/workflows/ci.yml) запускает `go vet`, тесты и проверяет сборку обоих образов на каждом pull request и push в `main`.

Проверка API:

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/v1/marketplaces
```

## Структура

- `cmd/api` — HTTP API.
- `cmd/worker` — процесс ежедневного сбора цен.
- `internal/domain` — независимые от инфраструктуры сущности и контракты.
- `marketplaces` — адаптеры источников и их реестр.

Новый маркетплейс реализует интерфейс `domain.Marketplace` и явно регистрируется в `internal/app/marketplaces.go`. Реальные HTTP-клиенты, PostgreSQL и планировщик появятся по мере реализации первого вертикального сценария. В стартовом каркасе подключённых маркетплейсов нет.
