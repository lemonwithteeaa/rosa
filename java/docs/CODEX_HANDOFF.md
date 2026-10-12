# Контекст для продолжения миграции РОСА

## Цель

Поэтапно перенести Windows-приложение РОСА с Go на Java, сохраняя действующие
расчёты, данные, интерфейсные сценарии и возможность отката. После каждого среза
обязательны автоматические тесты, сравнение с Go baseline, Windows-сборка и ручная
проверка.

## Репозиторий и контрольные точки

- Репозиторий: `lemonwithteeaa/rosa`.
- Рабочая ветка Java: `migration/java`.
- Неизменяемый Go baseline: тег `go-baseline-7.3.11.824-rc` на коммите `8afe8d8`.
- Архитектурный каркас Java: коммит `c70f55b`.
- Windows Java workflow: коммит `fd87e1b`.
- Последний успешный Java workflow: <https://github.com/lemonwithteeaa/rosa/actions/runs/38296998932>.

## Принятые решения

- Java 21 LTS, Gradle Wrapper 8.14.3, JavaFX, JUnit 5 и ArchUnit.
- Hexagonal architecture с направлением зависимостей внутрь.
- `rosa-domain` не зависит от UI, файлов, БД или Windows.
- `rosa-application` содержит use cases и ports.
- `rosa-persistence`, `rosa-compatibility`, `rosa-platform-windows` и
  `rosa-reports` являются adapters.
- `rosa-ui` зависит только от application/domain.
- `rosa-launcher` является composition root.
- Паттерны вводятся для реальной границы или вариативности, без pattern overuse.
- Рабочие данные нельзя конвертировать до появления backup и двустороннего
  round-trip теста.

Подробности находятся в `java/docs/adr/0001-hexagonal-modular-architecture.md` и
`java/docs/MIGRATION_PLAN.md`.

## Реализовано

- Девять Gradle-модулей и закреплённый checksum Gradle distribution.
- Первый вертикальный срез: UI получает сведения о Go baseline через use case и
  compatibility adapter.
- JavaFX preview с self-contained Windows runtime.
- Unit, compatibility, view-model и architecture tests.
- GitHub artifact `rosa-java-preview-windows-x64` содержит
  `ROSA-Java-Preview.exe` и встроенный Java runtime.
- Go baseline проверки `internal/authcrypto`, `nno_core` и эталон CO2 остаются
  зелёными.

## Известный технический долг Go

- Около 99 тысяч строк Go и почти весь код в `package main`.
- `main.go` около 7 620 строк; `wndProc` около 2 148 строк.
- Много глобального состояния и прямых Win32 вызовов.
- Полный исторический набор содержит 148 известных падений. Многие RC-тесты
  проверяют несовместимые старые версии, координаты UI и строки исходников.
- Исторические тесты нельзя слепо переносить или использовать для изменения
  текущего поведения. Сначала нужно извлекать из них актуальные бизнес-правила.

## Следующий этап

Сделать инвентаризацию форматов данных и первый read-only compatibility slice:

1. Найти все точки чтения SQLite, JSON, JSONL и зашифрованных файлов в Go.
2. Описать форматы и версии схем без изменения исходных данных.
3. Выбрать один безопасный формат, предпочтительно справочник или данные без
   персональных сведений.
4. Зафиксировать Go fixture и ожидаемую семантику.
5. Реализовать Java reader через port/adapter.
6. Добавить contract, malformed-input и round-trip-without-write tests.
7. Повторно запустить Java tests, ArchUnit, Go baseline checks и Windows packaging.

Не начинать с авторизации, DPAPI, записи рабочих данных или полного переноса UI.

## Команды проверки

```shell
cd java
./gradlew test --no-daemon
./gradlew :rosa-launcher:installDist --no-daemon
```

```shell
cd Source
go test -count=1 ./internal/authcrypto ./nno_core
python3 reference/co2_plan/verify_reference.py
```

