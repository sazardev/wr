# wr

Modo lector para la terminal: descarga una página, se queda con el artículo,
lo convierte a Markdown y lo muestra con **colores que siguen el tema de tu
terminal**, incluyendo los **bloques de código coloreados por lenguaje**.

*English: a terminal reader mode. It fetches a page, extracts the article,
converts it to Markdown and renders it using your terminal's own ANSI palette,
with per-language syntax highlighting for code blocks.*

```
wr https://ejemplo.com/articulo
```

Un solo archivo, sin instalación: [`uv`](https://docs.astral.sh/uv/) resuelve
las dependencias de Python en un entorno aislado la primera vez que lo corres.

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

1. `curl` descarga la página (usa el almacén de certificados del sistema, con
   verificación siempre activa).
2. Se toma `<article>` (o `<main>`, o `<body>`) y se quita lo que estorba:
   menús, pies de página, formularios, scripts, filas que son solo enlaces.
3. [`markdownify`](https://github.com/matthewwithanm/python-markdownify) lo pasa
   a Markdown. El lenguaje de cada bloque se lee de `data-language`, de clases
   `language-xx` / `lang-xx` / `highlight-source-xx` / `sourceCode xx`.
4. La prosa se colorea con [`bat`](https://github.com/sharkdp/bat) y cada bloque
   de código con [Pygments](https://pygments.org/), los dos con los **16 colores
   ANSI**, así que adoptan la paleta de tu terminal (gruvbox, nord, etc.) sin
   configurar nada.
5. Todo va a `less -R`.

## Requisitos

- [`uv`](https://docs.astral.sh/uv/getting-started/installation/)
- `curl`, `less`
- [`bat`](https://github.com/sharkdp/bat) (opcional: sin él se imprime el Markdown sin colorear la prosa)
- Una terminal con colores ANSI

Probado en Linux (WSL2, CachyOS) con Alacritty y Python 3.14 vía `uv`.
No lo he probado en macOS.

## Instalación

```sh
git clone https://github.com/sazardev/wr.git
ln -s "$PWD/wr/wr" ~/.local/bin/wr      # o cópialo a cualquier carpeta de tu PATH
```

## Uso

```sh
wr URL           # lee el artículo en un pager (q para salir, / para buscar)
wr -L URL        # sin las URL de los enlaces (solo el texto)
wr --md URL      # imprime el Markdown tal cual, para pipes o para glow
wr pagina.html   # también funciona con un archivo local
```

Variable de entorno: `WR_BG=light` si tu terminal tiene fondo claro (por defecto
asume oscuro).

## Límites

- **Extracción heurística.** Toma `<article>`/`<main>`; en páginas atípicas puede
  dejar ruido o cortar contenido.
- **Sin JavaScript.** Si la página dibuja su contenido con JS, saldrá vacía o
  incompleta.
- **Los enlaces no son navegables**: se ven como `[texto](url)`. Para navegar,
  un navegador de terminal como `w3m` sigue siendo la herramienta.
- **El lenguaje depende del HTML.** Si la página no lo declara, el bloque se
  muestra sin color (no se adivina, para no equivocarse).
- Los bloques `mermaid` y otros sin lexer en Pygments salen sin color.

## Seguridad

El contenido de la página no es de confianza. Antes de mostrarlo se eliminan los
caracteres de control (incluido `ESC`), para que una página no pueda inyectar
secuencias de escape en tu terminal.

## Licencia

[MIT](LICENSE)
