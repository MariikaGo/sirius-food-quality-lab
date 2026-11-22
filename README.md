# Сириус.Еда: тестирование Go API

Практическая 10. Реализованы unit-тесты бизнес-границ, интеграционные тесты с Testcontainers (новые PostgreSQL/Redis на запуск), контракт JSON-события OrderPaid и мутационная проверка Go-кода.

Запуск тестов: ./test.command при работающем Docker. Без SIRIUS_TEST_DB тесты сами создают временные контейнеры и удаляют их. Локальный результат: evidence/testcontainers-tests.txt. Мутации: python3 scripts/mutations.py; результат evidence/mutations.json. Это собственный Go-runner мутаций и Go-тест контракта, не Python mutmut и не Pact.

GitHub Actions: .github/workflows/test.yml запускает go test и go vet на push и pull_request. Статус настоящего удалённого CI и защиты ветки будет сохранён отдельно после проверки.

Карта файлов: internal/orders/service_unit_test.go — границы количества/баланса/доступности и однократный возврат; internal/app/testmain_test.go — временная инфраструктура; internal/app/*_test.go — настоящие интеграционные проверки; internal/eda/contract_test.go — сериализация события и ожидания consumer; contracts/OrderPaid.json — пример контракта; scripts/mutations.py — намеренные изменения условий и запуск тестов. Остальные папки содержат приложение с событиями, gRPC, кешем и наблюдаемостью из предыдущих работ.

Секреты хранятся только локально в .env и не отправляются в GitHub. Запуск приложения: ./start.command; нагрузка Locust будет подключена отдельным профилем Docker.
