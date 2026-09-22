# KWiki - лёгкая система для ведения вики с историей изменений на базе Git

> **Ранний этап разработки.**

## Сборка и Запуск

go build -o build/kwiki ./cmd/kwiki

./build/kwiki -repo ./data/wiki-content -db ./data/wiki.db -addr :8000
