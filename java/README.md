# РОСА Java

Поэтапная замена Windows-приложения РОСА. Исходная Go-версия зафиксирована тегом
`go-baseline-7.3.11.824-rc` и остаётся эталоном поведения и форматов данных.

## Архитектурные правила

- `rosa-domain` содержит только модель и бизнес-правила.
- `rosa-application` содержит сценарии и порты, но не знает об UI и адаптерах.
- `rosa-persistence`, `rosa-compatibility`, `rosa-platform-windows` и `rosa-reports`
  реализуют порты приложения.
- `rosa-ui` зависит только от application/domain.
- `rosa-launcher` является composition root и связывает реализации.
- Каждый переносимый сценарий сначала получает контрактный тест против Go baseline.

Проверка проекта:

```shell
./gradlew test
```

Запуск preview:

```shell
./gradlew :rosa-launcher:run
```

