# KWiki - a lightweight Git-backed wiki

[Русская версия](README-ru.md)

Latest Linux build: [download kwiki](https://github.com/magomedcoder/KWiki/releases/latest/download/kwiki).

```bash
chmod +x kwiki
```

## Server

```bash
./kwiki -data ./data -addr :8000 -pepper 'long-secret' -secure
```

| Flag      | Default      | Purpose                                                                  |
|-----------|--------------|--------------------------------------------------------------------------|
| `-data`   | `./data`     | main storage                                                             |
| `-addr`   | `:8000`      | HTTP address                                                             |
| `-pepper` | empty        | password hash secret, same as for `user add` and `user passwd`           |
| `-secure` | off          | secure cookie and strict transport header; requires HTTPS                |
| `-home`   | `README.md`  | branch root page                                                         |
| `-lang`   | `ru`         | default UI language (`ru`, `en`)                                         |


## Users

Set the password with `-password`. If the flag is empty, it is read hidden from the terminal.

The first user becomes an administrator. `-admin` also makes later users administrators.

```bash
./kwiki user add -data ./data -email kwiki@example.com -name KWiki -surname KWiki -password 'S3cure-Wiki-Pass' -pepper 'long-secret' -admin

./kwiki user passwd -data ./data -email kwiki@example.com -password 'N3w-Wiki-Password' -pepper 'long-secret'

./kwiki user block -data ./data -email editor@example.com

./kwiki user unblock -data ./data -email editor@example.com

./kwiki user delete -data ./data -email editor@example.com
```

| Command        | Flags                                                              |
|----------------|--------------------------------------------------------------------|
| `user add`     | `-data` `-email` `-name` `-surname` `-password` `-pepper` `-admin` |
| `user passwd`  | `-data` `-email` `-password` `-pepper`                             |
| `user block`   | `-data` `-email`                                                   |
| `user unblock` | `-data` `-email`                                                   |
| `user delete`  | `-data` `-email`                                                   |

`-pepper` for `user add`, `user passwd`, and the server must match. Otherwise existing passwords will not work.

---

## Docker

```bash
docker build -t kwiki .
docker run -p 8000:8000 -v kwiki-data:/var/lib/data kwiki -pepper 'long-secret'
```

Data lives in the `kwiki-data` volume. First user:

```bash
docker run --rm -v kwiki-data:/var/lib/data kwiki user add -data ./data -email kwiki@example.com -name KWiki -surname KWiki -password 'S3cure-Wiki-Pass' -pepper 'long-secret' -admin
```

---

## Build from source

Requires Go 1.27, Node.js 22, and Yarn 1.22.

```bash
yarn
yarn build:css
go build -o build/kwiki ./cmd/kwiki
```

`yarn build:css` must run before `go build`
