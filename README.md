# KWiki - лёгкая система для ведения вики на базе Git

Последняя сборка для Linux: [скачать kwiki](https://github.com/magomedcoder/KWiki/releases/latest/download/kwiki).

```bash
chmod +x kwiki
```

## Сервер

```bash
./kwiki -data ./data -addr :8000 -pepper 'длинный-секрет' -secure
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
./kwiki user add -data ./data -email kwiki@example.com -name KWiki -surname KWiki -password 'S3cure-Wiki-Pass' -pepper 'длинный-секрет' -admin

./kwiki user passwd -data ./data -email kwiki@example.com -password 'N3w-Wiki-Password' -pepper 'длинный-секрет'

./kwiki user block -data ./data -email editor@example.com

./kwiki user unblock -data ./data -email editor@example.com

./kwiki user delete -data ./data -email editor@example.com
```

| Команда        | Флаги                                                              |
|----------------|--------------------------------------------------------------------|
| `user add`     | `-data` `-email` `-name` `-surname` `-password` `-pepper` `-admin` |
| `user passwd`  | `-data` `-email` `-password` `-pepper`                             |
| `user block`   | `-data` `-email`                                                   |
| `user unblock` | `-data` `-email`                                                   |
| `user delete`  | `-data` `-email`                                                   |

`-pepper` у `user add`, `user passwd` и у сервера должен совпадать. Иначе уже сохранённые пароли не подойдут.

---

## Docker

```bash
docker build -t kwiki .
docker run -p 8000:8000 -v kwiki-data:/var/lib/data kwiki -pepper 'длинный-секрет'
```

Данные в томе `kwiki-data`. Первый пользователь:

```bash
docker run --rm -v kwiki-data:/var/lib/data kwiki user add -data ./data -email kwiki@example.com -name KWiki -surname KWiki -password 'S3cure-Wiki-Pass' -pepper 'длинный-секрет' -admin
```

---

## Сборка из исходников

Нужны Go 1.27, Node.js 22 и Yarn 1.22.

```bash
yarn
yarn build:css
go build -o build/kwiki ./cmd/kwiki
```

`yarn build:css` нужен до `go build`
