# WaiLauncher

Настольный лаунчер Minecraft на **Wails v2** (Go + Vue 3). Поддерживает ванильный запуск, модлоадеры (Fabric, Forge, NeoForge), отдельные сборки с собственными папками, установку модов и модпаков с Modrinth/CurseForge.

![WaiLauncher](build/appicon.png)

## Возможности

### Основы
- **Авторизация** — оффлайн-аккаунты и полный Microsoft OAuth (device code flow)
- **Версии** — манифест Mojang (релизы, снапшоты, old_beta/old_alpha), кэш для оффлайн-работы
- **Модлоадеры** — Fabric, Forge (включая manual processor steps), NeoForge (включая новый FML startup 21.3+), генерация merged version JSON
- **Сборки (instances)** — профили в стиле Modrinth App: своя версия + лоадер + папка на сборку; миграция legacy-папки `game/`
- **Java** — автоопределение нужной мажорной версии JDK и автозагрузка Temurin, проверка новых patch-выпусков

### Контент
- **Моды** — поиск и установка с Modrinth (моды, ресурспаки, шейдерпаки), вкл/выкл/удаление, проверка обновлений
- **Модпаки** — поиск и установка с Modrinth и CurseForge; импорт `.mrpack`/`.zip` из файла или перетаскиванием в окно
- **Каталог** — подборки популярных шейдеров и ресурспаков для активной сборки, установка в один клик
- **Миры, скриншоты, логи** — просмотр миров, галерея скриншотов, live-журнал игры с поиском (Ctrl+F)

### Отказоустойчивость и диагностика
- **Краш-репорты** — отдельная вкладка с содержимым `crash-reports/`
- **Анализ крашей** — распознание типичных причин (нехватка ОЗУ, конфликт модов, проблемы GPU) и понятное сообщение вместо «игра упала»

### Персонализация сборки
- **Настройки на сборку** — свой RAM, свой Java, JVM-аргументы, режим окна (окно/разрешение/фуллскрин) для каждой сборки отдельно
- **Клонирование** — дублирование сборки со всеми модами и настройками в один клик
- **Экспорт/импорт** — выгрузка сборки в `.mrpack`/`.zip` и установка из архива

### Система
- **Автообновление лаунчера** — проверка GitHub Releases при запуске, скачивание нового exe с прогресс-баром, замена бинарника, рестарт (включается в Настройках)
- **Discord Rich Presence** — показывает активную сборку и таймер игры (нужен Application ID из Discord Developer Portal)
- **Системный трей** — при запущенной игре: показать лаунчер / остановить игру / выход
- **Статистика времени** — «сегодня» и «всего» в сайдбаре, учитывается отдельно по дням

## Структура

```
├── main.go, app.go, instances.go, settings.go, update.go, tray.go  # Wails-bound API
├── internal/launcher/                          # ядро лаунчера
│   ├── manifest.go, types.go   # Mojang piston-meta
│   ├── install.go              # клиент, библиотеки, ассеты, natives
│   ├── jvm.go, launch.go       # classpath, аргументы, запуск процесса
│   ├── java.go                 # поиск/загрузка/обновление JDK
│   ├── loaders.go, loaderprofile.go, forgeinstall.go
│   ├── modrinth.go, modpacks.go, content.go, news.go
│   ├── discord.go              # Discord Rich Presence (IPC, без DLL)
│   └── auth/                   # аккаунты: offline + Microsoft OAuth
└── frontend/src/               # Vue 3 + TailwindCSS v4
    ├── pages/                  # Home, Instances, Mods, News, Settings
    └── components/             # TitleBar, SideBar, AccountsModal, LaunchOverlay
```

Данные хранятся в `%APPDATA%/WaiLauncher` (`versions/`, `libraries/`, `assets/`, `instances/`, `java/`, `cache/`, `settings.json`, `accounts.json`, `instances.json`).

## Разработка

Требуются [Go 1.25+](https://go.dev), [Node.js](https://nodejs.org) и [Wails CLI](https://wails.io/docs/gettingstarted/installation):

```powershell
wails dev      # live-reload: Vite-сервер + Go-бэкенд
```

## Сборка

```powershell
wails build    # собирает frontend и линкует exe в build/bin/WaiLauncher.exe
```

## Автообновление

Лаунчер сверяет текущую версию с последним релизом в GitHub. Чтобы потестить обновление вручную: соберите exe с более старой версией (константа `launcherVersion` в `app.go`), запустите его — при выходе новой версии появится модалка обновления. В разделе настроек можно включить/выключить автопроверку и проверить вручную.

## Лицензия

Проект распространяется под лицензией [GNU General Public License v3.0 (GPL-3.0)](LICENSE).

---

Собрано с ❤️ на Go + Vue 3 + Wails v2.
