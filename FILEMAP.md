# Карта исходных файлов

Это подсказка для поиска в VS Code (Cmd+P — файл, Cmd+F — функция). Сгенерированные protobuf-файлы вручную не редактируются. `cmd/launcher` и `scripts/infra.sh` унаследованы от нативного запуска; `start.command` использует Docker.

| Файл | Назначение и функции |
|---|---|
| `.dockerignore` | Настройки/документация .dockerignore |
| `.env.example` | Настройки/документация .env.example |
| `.github/workflows/test.yml` | Конфигурация .github/workflows/test.yml |
| `.gitignore` | Настройки/документация .gitignore |
| `Dockerfile` | Сборка контейнера |
| `cmd/api/main.go` | Точка запуска соответствующей программы; `main` |
| `cmd/delivery/main.go` | Точка запуска соответствующей программы; `main` |
| `cmd/demo/main.go` | Унаследованный демонстратор фоновых задач (работа 5); `must`, `call`, `login`, `main` |
| `cmd/events/main.go` | Точка запуска соответствующей программы; `main` |
| `cmd/grpcdemo/main.go` | Унаследованный демонстратор gRPC (работа 6); `must`, `call`, `login`, `main` |
| `cmd/launcher/main.go` | Точка запуска соответствующей программы; `main` |
| `cmd/loadcleanup/main.go` | Отмена фоновых задач только synthetic load-аккаунта; `main` |
| `cmd/scheduler/main.go` | Точка запуска соответствующей программы; `main` |
| `cmd/seed/main.go` | Точка запуска соответствующей программы; `main` |
| `cmd/worker/main.go` | Точка запуска соответствующей программы; `main` |
| `contracts/OrderPaid.json` | Конфигурация contracts/OrderPaid.json |
| `docker-compose.yml` | Конфигурация docker-compose.yml |
| `go.mod` | Версия Go и зависимости |
| `infra/grafana/dashboards/provider.yml` | Конфигурация infra/grafana/dashboards/provider.yml |
| `infra/grafana/dashboards/sirius.json` | Конфигурация infra/grafana/dashboards/sirius.json |
| `infra/grafana/datasources/prometheus.yml` | Конфигурация infra/grafana/datasources/prometheus.yml |
| `infra/prometheus.yml` | Конфигурация infra/prometheus.yml |
| `infra/rabbitmq/Dockerfile` | Сборка контейнера |
| `internal/app/app.go` | Сборка приложения и HTTP-маршруты; `New` |
| `internal/app/cache_test.go` | Тесты пакета internal/app; `TestCacheWarmupTTLInvalidation`, `TestCacheStampede`, `TestRedisFallback` |
| `internal/app/integration_test.go` | Тесты пакета internal/app; `setup`, `request` |
| `internal/app/jobs_test.go` | Тесты пакета internal/app; `fixtureJob`, `engineFor`, `TestJobsIdempotency`, `TestJobsRetryArchiveAndDLQ`, `TestScheduledStatisticsHandler` |
| `internal/app/rpc_test.go` | Тесты пакета internal/app; `TestRPCStreamsAndInternalKey`, `GetDelivery`, `TestRPCErrors`, `TestHTTPServiceUnavailable` |
| `internal/app/security_test.go` | Тесты пакета internal/app; `login`, `TestAuthPermissions`, `TestRateLimit`, `TestRefreshReplay`, `TestPasswordHashAndRole`, `TestRefundOnlyOnce`, `TestProductionDocsDisabled`, `strconvID`, `TestDeliveryOwnership` |
| `internal/app/testmain_test.go` | Тесты пакета internal/app; `TestMain`, `runTests` |
| `internal/auth/middleware.go` | JWT, роли, Redis rate limit, CORS; `Protect`, `Limit`, `CORS` |
| `internal/auth/repository.go` | SQL-доступ и транзакции; `Save`, `Rotate` |
| `internal/auth/router.go` | HTTP-хэндлеры и JSON входа/выхода; `Login`, `Refresh` |
| `internal/auth/security.go` | bcrypt, access JWT, случайные токены; `NewSecurity`, `Hash`, `Verify`, `Random`, `HashToken`, `Access`, `Decode` |
| `internal/auth/service.go` | Бизнес-правила и интерфейсы зависимостей; `pair`, `Login`, `Refresh` |
| `internal/config/config.go` | Настройки окружения; `Load`, `env`, `Int` |
| `internal/database/database.go` | Пул PostgreSQL и SQL-миграции; `Open`, `Migrate` |
| `internal/database/migrations/001_initial.sql` | SQL-миграция PostgreSQL |
| `internal/database/migrations/002_auth.sql` | SQL-миграция PostgreSQL |
| `internal/database/migrations/003_jobs.sql` | SQL-миграция PostgreSQL |
| `internal/database/migrations/004_events.sql` | SQL-миграция PostgreSQL |
| `internal/database/seed.go` | Учебные начальные данные; `Seed` |
| `internal/delivery/repository.go` | SQL-доступ и транзакции; `Transaction`, `Order`, `Courier`, `Active`, `notfound`, `scan`, `Insert`, `Delivery`, `Complete`, `ByOrder`, `Owner` |
| `internal/delivery/router.go` | HTTP-хэндлеры и JSON входа/выхода; `Assign`, `Get`, `Complete` |
| `internal/delivery/service.go` | Бизнес-правила и интерфейсы зависимостей; `Assign`, `Get`, `Complete` |
| `internal/deliverypb/delivery.pb.go` | Сгенерированный контракт protobuf/gRPC из proto/delivery.proto |
| `internal/deliverypb/delivery_grpc.pb.go` | Сгенерированный контракт protobuf/gRPC из proto/delivery.proto |
| `internal/docs/docs.go` | OpenAPI и интерфейс Swagger; `Mount`, `Spec`, `contains` |
| `internal/domain/domain.go` | Модели, деньги и общие ошибки; `MarshalJSON`, `UnmarshalJSON`, `ParseMoney`, `Error`, `Fail`, `Owner`, `Role`, `ID` |
| `internal/domain/domain_test.go` | Тесты пакета internal/domain; `TestMoney` |
| `internal/eda/contract_test.go` | Тесты пакета internal/eda; `TestOrderPaidMessageContract` |
| `internal/eda/events.go` | RabbitMQ, outbox и обработка доменных событий; `WithCorrelation`, `Correlation`, `ID`, `Record`, `logEvent`, `setup`, `send`, `PublishPending`, `Process`, `session`, `validType`, `Run` |
| `internal/jobs/jobs.go` | Фоновые задачи, повторы, идемпотентность и DLQ; `New`, `Close`, `Enqueue`, `AfterOrder`, `Publish`, `Process`, `Handle`, `OnError`, `Statistics`, `Worker` |
| `internal/menu/cache.go` | Cache-aside, TTL, инвалидирование, защита от stampede; `key`, `read`, `lock`, `All`, `ByID`, `write`, `Create`, `Update` |
| `internal/menu/repository.go` | SQL-доступ и транзакции; `scan`, `All`, `ByID`, `unique`, `Create`, `Update` |
| `internal/menu/router.go` | HTTP-хэндлеры и JSON входа/выхода; `All`, `Create`, `Update`, `Get` |
| `internal/menu/service.go` | Бизнес-правила и интерфейсы зависимостей; `All`, `Create`, `Update` |
| `internal/observe/observe.go` | Логи, HTTP-метрики и OTel; `SetPool`, `Metrics`, `Write`, `Install`, `WriteHeader`, `Write`, `Flush`, `HTTP` |
| `internal/orders/repository.go` | SQL-доступ и транзакции; `Transaction`, `User`, `Item`, `scan`, `Order`, `Insert`, `Balance`, `Status`, `ByID`, `All` |
| `internal/orders/router.go` | HTTP-хэндлеры и JSON входа/выхода; `Create`, `Get`, `Cancel`, `Paid`, `Kitchen` |
| `internal/orders/service.go` | Бизнес-правила и интерфейсы зависимостей; `Create`, `Get`, `Cancel`, `Paid` |
| `internal/orders/service_unit_test.go` | Тесты пакета internal/orders; `Transaction`, `ByID`, `All`, `User`, `Item`, `Order`, `Insert`, `Status`, `Balance`, `TestCreateBusinessBoundaries`, `TestCancelRefundOnce` |
| `internal/rpcdelivery/client.go` | Клиент gRPC и преобразование ошибок; `NewClient`, `Close`, `ctx`, `translate`, `Assign`, `Get`, `Complete`, `Track`, `Chat` |
| `internal/rpcdelivery/server.go` | Сервер протокола и проверка доступа; `NewServer`, `rpcError`, `actor`, `AssignCourier`, `CompleteDelivery`, `GetDelivery`, `TrackDelivery`, `ChatWithCourier` |
| `internal/transport/http.go` | Транспортные помощники HTTP; `Handle`, `JSON`, `Decode`, `Error`, `WithActor`, `Actor`, `PathID` |
| `internal/users/memory.go` | Подставное хранилище для unit-тестов; `NewMemory`, `ByID`, `ByEmail`, `Create`, `Topup` |
| `internal/users/repository.go` | SQL-доступ и транзакции; `Columns`, `Scan`, `ByID`, `ByEmail`, `Create`, `Topup` |
| `internal/users/router.go` | HTTP-хэндлеры и JSON входа/выхода; `Register`, `Topup`, `Me` |
| `internal/users/service.go` | Бизнес-правила и интерфейсы зависимостей; `Register`, `Topup` |
| `internal/users/service_test.go` | Тесты пакета internal/users; `TestServiceWithMemoryRepository` |
| `load/locustfile.py` | Сценарий нагрузки 7/2/1 для работы 10 |
| `load.command` | Короткий сценарий load |
| `loadtest/locustfile.py` | Унаследованный нагрузочный сценарий кеша (работа 4) |
| `locust.command` | Короткий сценарий locust |
| `proto/delivery.proto` | Контракт gRPC доставки |
| `scripts/bootstrap.py` | Вспомогательный сценарий bootstrap |
| `scripts/build.sh` | Вспомогательный сценарий build |
| `scripts/fault_demo.py` | Вспомогательный сценарий fault_demo |
| `scripts/infra.sh` | Вспомогательный сценарий infra |
| `scripts/load.py` | Вспомогательный сценарий load |
| `scripts/mutations.py` | Вспомогательный сценарий mutations |
| `scripts/proto.sh` | Вспомогательный сценарий proto |
| `start.command` | Короткий сценарий start |
| `stop.command` | Короткий сценарий stop |
| `test.command` | Короткий сценарий test |
