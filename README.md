# KWiki - лёгкая система для ведения вики с историей изменений на базе Git

> **Ранний этап разработки.**

## Сборка

```bash
go build -o build/kwiki ./cmd/kwiki
```

## Сервер

```bash
./build/kwiki -data ./data -addr :8000 -pepper 'длинный-секрет' -secure
```

| Флаг      | По умолчанию | Назначение                                                      |
|-----------|--------------|-----------------------------------------------------------------|
| `-data`   | `./data`     | основное хранилище                                              |
| `-addr`   | `:8000`      | адрес HTTP                                                      |
| `-pepper` | пусто        | секрет хеша паролей, тот же, что при `user add` и `user passwd` |
| `-secure` | выключен     | защищённая кука и строгий заголовок транспорта, нужен HTTPS     |
| `-home`   | `README.md`  | страница корня ветки                                            |

## Пользователи

Пароль задаётся флагом `-password`. Если флаг пустой, читает его скрыто из терминала.

Первый пользователь становится администратором. Флаг `-admin` делает администратором и следующих.

```bash
./build/kwiki user add -data ./data -email kwiki@example.com -name KWiki -surname KWiki -password 'S3cure-Wiki-Pass' -pepper 'длинный-секрет' -admin

./build/kwiki user passwd -data ./data -email kwiki@example.com -password 'N3w-Wiki-Password' -pepper 'длинный-секрет'

./build/kwiki user block -data ./data -email editor@example.com

./build/kwiki user unblock -data ./data -email editor@example.com

./build/kwiki user delete -data ./data -email editor@example.com
```

| Команда        | Флаги                                                              |
|----------------|--------------------------------------------------------------------|
| `user add`     | `-data` `-email` `-name` `-surname` `-password` `-pepper` `-admin` |
| `user passwd`  | `-data` `-email` `-password` `-pepper`                             |
| `user block`   | `-data` `-email`                                                   |
| `user unblock` | `-data` `-email`                                                   |
| `user delete`  | `-data` `-email`                                                   |

`-pepper` у `user add`, `user passwd` и у сервера должен совпадать. Иначе уже сохранённые пароли не подойдут.
