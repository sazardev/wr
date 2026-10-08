# wr

Modo lector para la terminal: descarga una página, se queda con el artículo,
lo convierte a Markdown y lo muestra con **los colores de tu terminal**
(la paleta ANSI de tu tema), incluyendo los **bloques de código coloreados por
lenguaje**.

*English: a terminal reader mode. It fetches a page, extracts the article,
converts it to Markdown and renders it with your terminal's own ANSI palette,
with per-language syntax highlighting for code blocks. Single Go binary.*

```
wr https://ejemplo.com/articulo
```

Un solo binario, sin dependencias en tiempo de ejecución (ni Python, ni `bat`,
ni `curl`).

## Por qué existe

Quería leer documentación y artículos técnicos sin salir de la terminal y sin
perder el código resaltado. Lo que probé antes no lo resolvía:

| Herramienta | Qué pasó |
|---|---|
| `w3m` / `lynx` | Muestran el HTML, pero el código sale sin color: ignoran `style` y `<font color>`. |
| `trafilatura --markdown` | Buena extracción, pero descarta el lenguaje de los bloques de código (``` sin lenguaje), así que no hay con qué colorear. |
| `bat -l markdown` | Colorea el Markdown, pero **no** el código dentro de los fences. |

`wr` junta las piezas y conserva el dato clave: el lenguaje de cada bloque.

## Cómo funciona

1. Descarga la página (con el almacén de certificados del sistema; la
   verificación TLS siempre está activa) y la pasa a UTF-8.
2. Toma `<article>` (o `<main>`, o `<body>`) y quita lo que estorba: menús,
   pies de página, formularios, scripts y filas que son solo enlaces.
3. Lee el lenguaje de cada bloque de `data-language` o de clases
   `language-xx` / `lang-xx` / `highlight-source-xx` / `sourceCode xx`, y lo
   convierte a Markdown con
   [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown).
4. Colorea los bloques con [chroma](https://github.com/alecthomas/chroma) y la
   prosa con reglas propias, siempre con los **16 colores ANSI**: la terminal
   decide los tonos, así que se adapta a gruvbox, nord, etc. sin configurar nada.
5. Lo muestra en `less -R` (si no hay `less` o no es una terminal, lo imprime).

## Instalación

Con Go 1.26 o superior:

```sh
go install github.com/sazardev/wr@latest
```

o desde el código:

```sh
git clone https://github.com/sazardev/wr.git
cd wr
go build -ldflags="-s -w" -o ~/.local/bin/wr .
```

El binario pesa unos 12 MB porque chroma incluye todos sus lexers.

Probado en Linux (WSL2, CachyOS) con Alacritty y Go 1.27. No lo he probado en
macOS ni en Windows nativo.

## Uso

```sh
wr URL           # lee el artículo en un pager (q para salir, / para buscar)
wr -L URL        # sin las URL de los enlaces (solo el texto)
wr --md URL      # imprime el Markdown tal cual, para pipes
wr pagina.html   # también funciona con un archivo local
```

## Velocidad

Medido sobre un artículo de ~290 KB (5 corridas, WSL2):

| | Tiempo |
|---|---|
| `wr` sobre un archivo local | ~11 ms |
| `wr` descargando el artículo | ~0.35–0.55 s (casi todo es red) |

La primera versión, en Python, tardaba ~84 ms sobre un archivo local y
~1.0–1.5 s con red.

## Límites

- **Extracción heurística.** Toma `<article>`/`<main>`; en páginas atípicas puede
  dejar ruido o cortar contenido.
- **Sin JavaScript.** Si la página dibuja su contenido con JS, saldrá vacía o
  incompleta.
- **Los enlaces no son navegables**: se ven como `texto (url)`. Para navegar,
  un navegador de terminal como `w3m` sigue siendo la herramienta.
- **El lenguaje depende del HTML.** Si la página no lo declara, el bloque se
  muestra sin color (no se adivina, para no equivocarse). Los bloques `mermaid`
  y otros sin lexer en chroma también salen sin color.
- **El Markdown se estiliza con reglas simples** (títulos, negritas, cursivas,
  código en línea, listas, citas, tablas). Cubre lo habitual, no todo Markdown.

## Seguridad

El contenido de la página no es de confianza. Antes de mostrarlo se eliminan los
caracteres de control (incluido `ESC`), para que una página no pueda inyectar
secuencias de escape en tu terminal.

## Licencia

[MIT](LICENSE)
