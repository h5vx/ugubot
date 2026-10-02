# Деплой

`deploy.py` готовит удалённый хост к запуску стека через docker compose. Исходники на хост не копируются.

1. Локально собирает образ `ugubot:<git describe>` и сохраняет его в `deploy/.build/`.
2. Загружает образ на хост и делает `docker load`, только если там нет образа с таким же id.
3. Кладёт в `ugubot_dir` (по умолчанию `/opt/ugubot`) два файла, которые перезаписываются при каждом деплое:
   - `docker-compose.yml`;
   - `.env` с тегом образа, uid/gid и паролем PostgreSQL. Пароль генерируется один раз и дальше сохраняется.
4. Создаёт `settings.toml` и `.secrets.toml`, если их ещё нет. Права `600`, существующие файлы не трогаются.

Что нужно:
- локально: `docker` и `pyinfra` 3;
- на хосте: `docker` с плагином compose. У SSH-пользователя должен быть доступ к docker, иначе запускайте с `--sudo`.

```sh
cp deploy/inventory.example.py deploy/inventory.py   # указать хост и настройки
pyinfra deploy/inventory.py deploy/deploy.py
ssh host 'cd /opt/ugubot && $EDITOR settings.toml && docker compose up -d'
```

Если в инвентаре задано `ugubot_up = True`, стек запускается в конце деплоя. Это удобно для обновлений: в `.env` меняется тег образа, и `up -d` пересоздаёт контейнеры.
