# WaiLauncher

![Version](https://img.shields.io/badge/version-1.1.2-brightgreen)
![Go](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/wails-v2-red)
![Vue](https://img.shields.io/badge/vue-3.x-4FC08D?logo=vuedotjs&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6)
![License](https://img.shields.io/badge/license-GPL--3.0-007ec6)

Современный, быстрый и функциональный настольный лаунчер Minecraft на **Wails v2** (Go + Vue 3). Поддерживает запуск ванильной игры, модлоадеров (Fabric, Forge, NeoForge, Quilt), изоляцию сборок с собственными папками, кастомизацию скинов/плащей с 3D-предпросмотром, а также установку модов и модпаков напрямую из **Modrinth**, **CurseForge** и **FTB (Feed The Beast)** с функцией **автообновления сборок в 1 клик**.

![WaiLauncher](build/appicon.png)

## Возможности

### 👤 Аккаунты и 3D Кастомизация
- **Авторизация** — оффлайн-аккаунты и официальный Microsoft OAuth (device code flow) с автоматическим обновлением токенов.
- **3D-предпросмотр скина** — интерактивный рендеринг скина персонажа на Three.js (`skinview3d`) с поддержкой анимаций (ходьба, бег, махание рукой), вращения и управления камерой.
- **Смена скина** — загрузка локальных `.png` файлов или по URL, переключение типа модели рук (**Classic 4px** / **Slim 3px**), прямая синхронизация с серверами Mojang Services API для Microsoft-аккаунтов.
- **Галерея и смена плащей** — выбор плащей из встроенной галереи (Minecon 2011–2016, 15th Anniversary, Cherry Blossom, Vanilla, Migrator, TikTok, Twitch, OptiFine), управление официальными плащами Mojang и загрузка своих кастомных текстур.

### 📦 Каталог модов и сборок (Modrinth, CurseForge & FTB)
- **Моды, ресурспаки и шейдеры** — полнотекстовый поиск и установка напрямую из двух крупнейших каталогов: **Modrinth** и **CurseForge**.
- **Сборки Feed The Beast (FTB)** — интеграция с официальной базой модов и сборок FTB App.
- **Автообновление сборок в 1 клик** — автоматическое отслеживание обновлений установленных сборок от авторов на Modrinth, CurseForge и FTB с сохранением миров (`saves/`), скриншотов и персональных настроек (`options.txt`).
- **Разрешение зависимостей в 1 клик** — автоматический анализ и предложение установки недостающих библиотек для выбранных модов.
- **Импорт готовых сборок** — автоматическое обнаружение и импорт установленных сборок из Modrinth App, CurseForge, Prism, MultiMC, ATLauncher и FTB App, а также импорт `.mrpack` и `.zip`.
- **Менеджер контента** — включение, отключение, удаление и пакетная проверка обновлений установленных модов.

### 🛠️ Изоляция и управление сборками (Instances)
- **Профили сборок** — каждая сборка полностью изолирована в собственной директории.
- **Смена версий и лоадеров** — быстрое переключение версии Minecraft (релизы, снапшоты) и модлоадеров (**Vanilla**, **Fabric**, **Forge**, **NeoForge**, **Quilt**) прямо из настроек сборки.
- **Гибкая конфигурация** — индивидуальное выделение RAM, выбор Java Runtime, JVM-аргументы, сервер для автоподключения, настройки окна и разрешения.
- **Клонирование и экспорт** — дублирование сборок со всеми модами и выгрузка в `.mrpack`/`.zip`.
- **Миры, скриншоты, логи** — встроенный менеджер миров, галерея скриншотов с полноэкранным просмотром и live-консоль логов игры.

### ☕ Автоматизация Java и производительность
- **Управление Java (OpenJDK)** — автоматическое определение требуемой версии (Java 8, 16, 17, 21), скачивание с Adoptium Temurin и проверка патчей.
- **Диагностика крашей** — отдельная вкладка с просмотром `crash-reports/` и интеллектуальным анализом причин сбоя (нехватка памяти, конфликты модов, ошибки GPU драйвера).

### 🌐 Интеграции и интерфейс
- **Discord Rich Presence** — отображение названия сборки, версии и времени игры в профиле Discord.
- **Системный трей** — управление игрой в фоновом режиме через иконку в трее (показать лаунчер, остановить игру, выход).
- **Многоязычность** — интерфейс на русском и английском языках.
- **Автообновление** — проверка и установка новых версий лаунчера через GitHub Releases.

---

## Структура проекта

```
├── main.go, app.go, instances.go, settings.go, update.go, tray.go  # Wails-bound API
├── internal/launcher/                          # Ядро лаунчера
│   ├── manifest.go, types.go   # Манифест Mojang piston-meta
│   ├── install.go              # Клиент, библиотеки, ассеты, natives
│   ├── jvm.go, launch.go       # Classpath, аргументы JVM, запуск процесса
│   ├── java.go                 # Поиск, установка и проверка версий OpenJDK
│   ├── loaders.go, loaderprofile.go, forgeinstall.go  # Лоадеры Fabric, Forge, NeoForge
│   ├── modrinth.go, curseforge.go, modpacks.go, content.go  # Каталоги и контент
│   ├── discord.go              # Discord Rich Presence (native IPC)
│   └── auth/                   # Аккаунты (Offline + Microsoft OAuth, скины, плащи)
└── frontend/src/               # Vue 3 + TailwindCSS
    ├── pages/                  # Home, Instances, Mods, News, Settings
    └── components/             # TitleBar, SideBar, AccountsModal, SkinViewer3D, Modals
```

Данные лаунчера по умолчанию хранятся в `%APPDATA%/WaiLauncher` (`versions/`, `libraries/`, `assets/`, `instances/`, `java/`, `cache/`, `settings.json`, `accounts.json`, `instances.json`).

---

## Разработка

Для разработки требуются [Go 1.25+](https://go.dev), [Node.js](https://nodejs.org) и [Wails CLI](https://wails.io/docs/gettingstarted/installation):

```powershell
# Запуск в режиме разработки с hot-reload
wails dev
```

## Сборка релиза

```powershell
# Сборка готового исполняемого файла
wails build
```
Скомпилированный файл будет находиться в директории `build/bin/WaiLauncher.exe`.

---

## Лицензия

Проект распространяется под лицензией [GNU General Public License v3.0 (GPL-3.0)](LICENSE).

---

Собрано с ❤️ на Go + Vue 3 + Wails v2.
