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

`Dockerfile` собирает статические Go-бинарники в Alpine и помещает их в отдельные минимальные `scratch`-образы: `api` и `worker`. В них также есть CA-сертификаты для HTTPS-запросов будущих адаптеров. В GitHub Actions workflow [ci.yml](.github/workflows/ci.yml) запускает `go vet` и тесты внутри `golang:1.24-alpine`, а также проверяет сборку обоих образов на каждом pull request и push в `main`.

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

Новый маркетплейс реализует интерфейс `domain.Marketplace` и явно регистрируется в `internal/app/marketplaces.go`. Для разбора ссылок предусмотрен отдельный `domain.LinkResolver`. Сейчас зарегистрирован экспериментальный адаптер Ozon; PostgreSQL, сбор по расписанию и уведомления — следующие итерации. Пока worker только перечисляет зарегистрированные источники.

## Ozon: первая итерация

Цена извлекается из JSON в `data-state` элемента
`div[id^="state-webPrice-"][data-state]`. `price` — обычная цена,
`cardPrice` — цена по карте Ozon. Режим выбирается явно: `standard` (по умолчанию)
или `card`; для реестра — переменной `OZON_PRICE_MODE`.
Селектор и поля встречаются в [публичном примере работы с разметкой Ozon](https://gist.github.com/br4instormer/24b029f34d00359bb4c0adec62dd5bb9).
Это недокументированный формат сайта, не официальный API.

Парсер проверяет canonical URL товара, заголовок, наличие и согласованность
виджетов. Старая цена, рассрочка и рекомендации не используются. Цены переводятся
в копейки без float; поддержаны обычные и неразрывные пробелы, запятая и точка.
Отсутствующий `cardPrice` не заменяется обычной ценой. `Available=false` означает
отсутствие товара, а не бесплатный товар или скидку до нуля.

Проверить одну карточку без БД и уведомлений:

```sh
go run ./cmd/inspect -url 'https://www.ozon.ru/product/1463825680/'
go run ./cmd/inspect -url 'https://www.ozon.ru/product/1463825680/' -price card
```

Проверить парсер на модельной разметке:

```sh
go run ./cmd/inspect -url 'https://www.ozon.ru/product/1463825680/' \
  -html marketplaces/ozon/testdata/1463825680.html
go test ./marketplaces/ozon -v
```

Фикстура использует товар «Дрип-кофе Sibaristica Mega Drip Max» (1463825680),
но **это синтетический пример с вымышленными тестовыми ценами, не снимок сайта**.
На проверке 2026-09-14 Ozon вернул HTTP 403 / Antibot Captcha. Работу на текущей
живой разметке пока не подтверждали. Парсер возвращает ошибку при блокировке
или неизвестном формате, не пытается обходить CAPTCHA. Для проверки реального
DOM можно передать сохранённый HTML через `-html`.

Пока отслеживается анонимная публичная цена конкретного артикула: продавец и
регион доставки не закреплены, это явно отражено в `Offer.context`.
Перед включением уведомлений эти условия нужно стабилизировать; иначе смена
продавца или региона может выглядеть как изменение цены.

## Браузерный режим: chromedp

`inspect -browser` загружает карточку через Chrome/Chromium, ждёт появления
ценового виджета после JavaScript и передаёт HTML тому же Go-парсеру.
Node.js не нужен. Chrome/Chromium устанавливается отдельно; браузер не входит
в текущие `scratch`-образы API и worker. В этой итерации режим доступен только
в CLI `inspect`, обычный HTTP-адаптер не меняется.

```sh
go run ./cmd/inspect -browser -url 'https://www.ozon.ru/product/1463825680/'
```

Путь к браузеру можно передать явно, например на macOS с Яндекс Браузером:

```sh
go run ./cmd/inspect -browser \
  -chrome '/Applications/Yandex.app/Contents/MacOS/Yandex' \
  -url 'https://www.ozon.ru/product/1463825680/'
```

По умолчанию открывается окно с отдельным временным профилем. `-headless`
отключает окно, `-profile .browser-profile` сохраняет отдельный профиль между
запусками. Не указывайте основной личный профиль: каталог может содержать
cookies и другие данные сессии. `.browser-profile/` исключён из Git.
`-html` и `-browser` нельзя использовать одновременно.

При CAPTCHA или заголовке блокировки возвращается `ErrBlocked`; её прохождение
не автоматизировано. Если виджет не появился за 30 секунд, возвращается ошибка
разметки. Весь вызов CLI ограничен 60 секундами. Браузерный режим сам по себе
не гарантирует доступ к Ozon.

Интеграционные тесты запускают браузер только при явном `CHROME_BIN` и читают
локальный HTTP-сервер: обычную страницу, динамический виджет и антибот.

```sh
CHROME_BIN='/Applications/Yandex.app/Contents/MacOS/Yandex' \
  go test ./marketplaces/ozon -run TestBrowserLoader -v
```

### Chromium в Docker

Отдельный target `browser` содержит Alpine, Chromium, шрифты, сертификаты и
статический Go-бинарник `inspect`. Node.js не устанавливается. Контейнер
работает от непривилегированного пользователя, `tini` управляет дочерними
процессами, для Chromium выделено 256 МБ `/dev/shm` в Compose.

```sh
docker compose --profile browser run --build --rm browser
# Или другой товар:
docker compose --profile browser run --build --rm browser \
  -url 'https://www.ozon.ru/product/1463825680/' -price card
```

Sandbox Chromium оставлен включённым. Среда Docker должна разрешать работу
его sandbox (user namespaces); на хостах с ограничениями запуск может завершиться
ошибкой `No usable sandbox` / `Operation not permitted`. Привилегированный режим
и `--no-sandbox` автоматически не включаются. Проверку запуска контейнера нужно
проводить на целевом Docker-хосте; сборка образа проверяется отдельно в CI.

Локально chromedp проверен с Яндекс Браузером на модельных страницах.
Живая карточка Ozon 2026-09-14 вернула антибот и в браузерном режиме;
переход на Chromium в контейнере сам по себе не устраняет блокировку.
