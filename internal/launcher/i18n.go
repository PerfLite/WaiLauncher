package launcher

import "fmt"

// T returns a localized backend message for lang ("ru" or "en"),
// falling back to Russian and then to the key itself.
func T(lang, key string, args ...any) string {
	dict, ok := messages[lang]
	if !ok {
		dict = messages["ru"]
	}
	s, ok := dict[key]
	if !ok {
		s = messages["ru"][key]
	}
	if s == "" {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

var messages = map[string]map[string]string{
	"ru": {
		"launch.busy":         "уже выполняется запуск",
		"launch.cancelled":    "Отменено",
		"launch.error":        "Ошибка: %s",
		"launch.crash":        "Игра завершилась с ошибкой",
		"launch.done":         "Игра завершена",
		"dialog.java":         "Выберите javaw.exe",
		"dialog.dataDir":      "Выберите папку данных",
		"err.no_client":       "версия %s не имеет клиентского jar (возможно, это модлоадер)",
		"err.no_java":         "Java не найдена: установите Java 17+ или укажите путь в настройках",
		"err.no_args":         "версия %s: нет аргументов запуска",
		"err.manifest":        "манифест версий: %w",
		"err.not_found":       "версия %s не найдена в манифесте",
		"err.meta":            "метаданные версии: %w",
		"err.java_start":      "запуск java: %w",
		"err.java_dl":         "не удалось установить Java %d: %w",
		"err.no_instance":     "сборка не найдена",
		"err.pick_version":    "сначала выберите версию",
		"err.loader_versions": "не удалось получить версии загрузчика: %w",
		"err.loader_none":     "нет версий %s для Minecraft %s",
		"err.installer":       "инсталлер %s: %w",
		"err.processor":       "шаг установки %s: %w",
		"err.auth_failed":     "ошибка авторизации: %w",
		"err.no_account":      "аккаунт не выбран или не найден",
		"news.tag.snapshot":   "Снапшот",
		"news.tag.services":   "Сервисы",
		"news.tag.java":       "Java Edition",
		"news.tag.news":       "Новости",
		"news.more":           "Подробности — в статье на minecraft.net.",
		"crash.oom":           "Краш: не хватило оперативной памяти (OutOfMemory). Увеличьте ОЗУ для сборки.",
		"crash.mods":          "Краш: конфликт или несовместимость модов. Отключите последние добавленные моды.",
		"crash.gpu":           "Краш: видеокарта не поддерживается (GL/GLFW). Обновите драйверы GPU.",
		"crash.glow":          "Краш: GPU слишком слабая для этой версии игры.",
		"crash.detail":        "Краш: %s — подробности во вкладке «Журнал»",
	},
	"en": {
		"launch.busy":         "a launch is already in progress",
		"launch.cancelled":    "Cancelled",
		"launch.error":        "Error: %s",
		"launch.crash":        "The game crashed",
		"launch.done":         "Game finished",
		"dialog.java":         "Select javaw.exe",
		"dialog.dataDir":      "Select data folder",
		"err.no_client":       "version %s has no client jar (maybe a modloader)",
		"err.no_java":         "Java not found: install Java 17+ or set the path in settings",
		"err.no_args":         "version %s: no launch arguments",
		"err.manifest":        "version manifest: %w",
		"err.not_found":       "version %s not found in the manifest",
		"err.meta":            "version metadata: %w",
		"err.java_start":      "starting java: %w",
		"err.java_dl":         "failed to install Java %d: %w",
		"err.no_instance":     "build not found",
		"err.pick_version":    "select a version first",
		"err.loader_versions": "failed to fetch loader versions: %w",
		"err.loader_none":     "no %s versions for Minecraft %s",
		"err.installer":       "installer %s: %w",
		"err.processor":       "install step %s: %w",
		"err.auth_failed":     "authentication failed: %w",
		"err.no_account":      "no account selected or found",
		"news.tag.snapshot":   "Snapshot",
		"news.tag.services":   "Services",
		"news.tag.java":       "Java Edition",
		"news.tag.news":       "News",
		"news.more":           "Details — in the article on minecraft.net.",
		"crash.oom":           "Crash: ran out of memory (OutOfMemory). Increase RAM for this build.",
		"crash.mods":          "Crash: mod conflict or incompatibility. Disable recently added mods.",
		"crash.gpu":           "Crash: GPU/graphics not supported (GL/GLFW). Update your GPU drivers.",
		"crash.glow":          "Crash: your GPU is too weak for this game version.",
		"crash.detail":        "Crash: %s — see the Logs tab for details",
	},
}
