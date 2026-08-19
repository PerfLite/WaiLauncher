# Промт для генерации иконки WaiLauncher

Отдай этот промт нейросети для генерации изображений (Midjourney, DALL·E, Ideogram, Kandinsky и т.п.), чтобы получить иконку для лаунчера.

## Айдентика проекта (на чём основан промт)

| Элемент | Значение |
|---|---|
| Название | WaiLauncher (лого в тайтлбаре: «WAI LAUNCHER») |
| Что это | Лаунчер Minecraft (тёмный UI, Go + Wails + Vue) |
| Фон | `#07090d` |
| Панели | `#10151c`, `#151c26`, `#1b2431` |
| Текст | `#e8eef4` |
| Акцент (зелёный) | `#55d24a` |
| Тёмно-зелёный | `#3aa832` |
| Свечение | `rgba(85, 210, 74, 0.35)` |
| Стиль | Тёмный минимализм + пиксель-арт мотивы (шрифт заголовков Press Start 2P), неоновые зелёные акценты |

## Основной промт

```
App icon for a Minecraft launcher called "WaiLauncher". A modern rounded-square
desktop app icon, dark theme. Deep charcoal-navy background (#07090d to #151c26
gradient). Centered motif: a stylized blocky letter "W" built from
Minecraft-like cubes, glowing bright green (#55d24a) with a soft neon glow
(rgba(85,210,74,0.35)), subtle darker green (#3aa832) shading on the cube
edges. Slight pixel-art influence, crisp geometric edges, minimal detail so the
icon stays readable at 16×16 px. No text, no letters other than the "W" motif,
no watermark. Flat modern game-launcher style, similar to official game
launcher icons. High contrast, centered composition, square 1:1 format.
```

## Варианты центральной фигуры

Замени описание мотивa в промте на один из вариантов:

1. **Буква W из кубов** — базовый вариант (в промте выше).
2. **Крипер + W** — `A creeper face subtly formed inside a blocky letter W, glowing green…`
3. **Кирка + кнопка Play** — `A diamond pickaxe crossed over a glowing green play-button triangle…`
4. **Блок травы** — `A Minecraft grass cube floating with a green neon glow underneath…`

## Технические требования (добавь в конец промта)

```
Output: square 1024×1024, subject centered with ~10% padding, no rounded
corners baked in (corners will be masked later), solid dark background filling
the whole canvas (no transparency artifacts).
```

## Что делать с результатом

1. Выбрать лучший вариант, обрезать/привести к квадрату.
2. Сконвертировать в `.ico` с наборами размеров: **16, 24, 32, 48, 64, 256 px**
   (например, онлайн-конвертером PNG→ICO или ImageMagick:
   `magick icon.png -define icon:auto-resize=16,24,32,48,64,256 icon.ico`).
3. Заменить файл `build\windows\icon.ico` — именно его Wails вшивает в `.exe`.
4. Пересобрать лаунчер:

   ```powershell
   wails build -clean -o WaiLauncher.exe
   ```
