# Promoción y distribución de wr

Dónde publicar `wr`, qué texto usar en cada sitio y qué ya está hecho en el
repositorio. Todo lo de esta lista es gratuito. Las tareas marcadas **manual**
necesitan tu cuenta (no se pueden hacer desde aquí); el resto ya está hecho o
es un comando que puedes lanzar tal cual.

## Hecho el 13 de octubre de 2026

- **Google Search Console**: propiedad `https://sazardev.github.io/wr/` verificada
  (archivo `google6e32854daef2e851.html`), sitemap-index y sitemap-0 enviados,
  home y `/docs/guide/start/` en cola de indexación.
- **Bing Webmaster Tools**: cuenta conectada con Google, sitio importado desde
  GSC y `sitemap-index.xml` con estado **Success**.
- **IndexNow**: 50 URLs enviadas en cada deploy (Bing, Yandex, Seznam, Naver, Yep).
- **Terminal Trove**: formulario enviado (categorías tui, cli, web, markdown,
  charm, linux, macos; captura real en `/assets/wr-preview.png`).
- **Console.dev**: email de propuesta enviado a hello@console.dev desde
  cerberusprogrammer@gmail.com.
- **Awesome lists**: PR a [awesome-tuis #941](https://github.com/rothgar/awesome-tuis/pull/941),
  [Awesome-Linux-Software #7](https://github.com/DimitrisPa/Awesome-Linux-Software/pull/7)
  (fork activo; el original está archivado) y
  [terminals-are-sexy #418](https://github.com/k4m4/terminals-are-sexy/pull/418).
- **Homebrew**: tap propio listo → `brew install sazardev/tap/wr`.
- **pkg.go.dev / libraries.io / deps.dev**: indexados.

Pendiente y bloqueado por sesión (hazlo tú en 2 minutos):

- **Show HN**: la sesión de `737max` caducó; entra en
  https://news.ycombinator.com/submit, pega el título y la URL del apartado 2 y
  publica el primer comentario. Mejor martes–jueves 14:00–16:00 UTC.
- **Reddit**: pide "prove your humanity"; resuelve el captcha e inicia sesión.
- **GitHub social preview**: repo → Settings → Social preview → sube
  `web/public/assets/og.png` (no hay API para esto).
- **dev.to, Product Hunt, AlternativeTo, LibHunt**: sin sesión en el navegador.
- **Golang Weekly**: el formulario de Cooperpress está caído (Airtable borrado);
  menciónalos en X/Mastodon o responde a su newsletter.

## Lo que ya está hecho

| Cosa | Dónde |
|---|---|
| Sitemap + robots.txt | https://sazardev.github.io/wr/sitemap-index.xml · /robots.txt |
| RSS de releases | https://sazardev.github.io/wr/rss.xml |
| `llms.txt` para asistentes | https://sazardev.github.io/wr/llms.txt |
| Documentación completa en un archivo | https://sazardev.github.io/wr/llms-full.txt |
| JSON-LD (SoftwareApplication, WebSite, TechArticle, BreadcrumbList) | todas las páginas |
| Tarjeta social PNG 1200×630 | web/public/assets/og.png |
| Captura real del lector | web/public/assets/wr-preview.png |
| IndexNow automático en cada deploy | `.github/workflows/docs.yml` |
| Topics del repo | browser, bubbletea, cli, golang, tui, … |
| pkg.go.dev | https://pkg.go.dev/github.com/sazardev/wr |

IndexNow avisa a Bing, Yandex, Seznam, Naver y Yep de cada deploy. Manualmente:
`cd web && npm run indexnow`. La tarjeta social se regenera con `npm run og`.

## 1. Indexación en buscadores (manual, una vez)

Google no usa IndexNow: necesita Search Console.

1. **Google Search Console** — https://search.google.com/search-console
   - Añade la propiedad con *Prefijo de URL*: `https://sazardev.github.io/wr/`
   - Verifica con la etiqueta HTML: pega el `<meta name="google-site-verification" …>`
     en `web/src/layouts/Base.astro` (dentro de `<head>`) y haz push. También
     vale subir el archivo `googleXXXX.html` a `web/public/`.
   - Envía `https://sazardev.github.io/wr/sitemap-index.xml` y pide indexación
     de la home y de `/docs/guide/start/`.
2. **Bing Webmaster Tools** — https://www.bing.com/webmasters
   - «Importar desde Google Search Console» (dos clics) o verificación propia.
3. **Yandex Webmaster** (opcional) — https://webmaster.yandex.com
4. Comprobar en unas semanas: `site:sazardev.github.io/wr` en Google y Bing.
   Para consultas concretas usa Search Console → Rendimiento.

Extra: en GitHub → Settings → Social preview, sube `web/public/assets/og.png`
(el repo no permite cambiarlo por API; es manual y se ve en cada enlace
compartido de GitHub).

## 2. Lanzamiento (una vez, con timing)

El mejor momento: martes–jueves, 14:00–16:00 UTC (mañana en EE. UU., tarde en
Europa). Publica primero en Hacker News; si funciona, ese mismo día en Reddit.

### Show HN

- URL: https://news.ycombinator.com/submit
- Título: `Show HN: wr – Reader mode and browser for the terminal`
- URL a enviar: `https://sazardev.github.io/wr/`
- Primer comentario (publícalo tú al momento, con contexto y honestidad):

```
Hi HN — I wanted to read docs and technical articles in the terminal without
losing highlighted code, and everything I tried failed one way or another
(w3m/lynx render HTML but code is plain; trafilatura drops the language of
fenced blocks; bat colors Markdown but not the code inside).

wr fetches a page, extracts the article, converts it to real Markdown (headings,
lists, tables — the syntax is gone, it is typeset), highlights code per
language, and gives you a browsing experience on top: clickable links (or f
labels), back/forward with cache and scroll restore, address bar that searches
the web, history, bookmarks, section index, search-as-you-type, mouse, and
every setting and keybinding editable from inside the program. It wears your
terminal's 16-color palette, so it follows your theme with zero config.
Single static Go binary (~14 MB), MIT.

Pages that need JS are rendered through an installed headless Chromium when
one is there; otherwise it says so instead of showing an empty shell.

Docs (with a recorded demo): https://sazardev.github.io/wr/
Source: https://github.com/sazardev/wr

Happy to answer anything about the extraction, the TUI or the design system
behind the docs site.
```

### Reddit

Publica **un post nativo** (texto + 2–3 capturas o el GIF), no solo un enlace;
muchos subs lo penalizan. Lee las reglas de cada uno antes.

- r/golang — https://www.reddit.com/r/golang/submit (flair *Showcase*)
  Título: `wr: a terminal reader and browser in Go (Bubble Tea) — formatted Markdown, code highlighting, full browsing`
- r/commandline — https://www.reddit.com/r/commandline/submit
  Título: `wr — reader mode and browser for the terminal: formatted Markdown, highlighted code, links, history, bookmarks`
- r/Terminal — https://www.reddit.com/r/Terminal/submit
  Título: `I built wr: read any article in your terminal with real code highlighting and a full browsing experience`
- r/linux — https://www.reddit.com/r/linux/submit (reglas estrictas de autopromoción: un solo post, aporta contexto)
  Título: `wr — read the web in the terminal (open source, MIT, single Go binary)`
- r/opensource — https://www.reddit.com/r/opensource/submit
- r/coolgithubprojects — https://www.reddit.com/r/coolgithubprojects/submit
  Título: `wr — reader mode and browser for the terminal (Go, MIT)`
- r/sideproject — https://www.reddit.com/r/sideproject/submit
- En español: r/programacion — https://www.reddit.com/r/programacion/submit
  Título: `wr: lector y navegador para la terminal — Markdown formateado, código con color y navegación completa`

Cuerpo sugerido (adáptalo al sub):

```
wr is a reader and browser for the terminal. It fetches a page, extracts the
article and opens it in an animated TUI: properly formatted Markdown (real
headings, lists, tables), per-language syntax highlighting inside labeled
frames, clickable links, back/forward with cached pages and scroll restore,
address bar that searches the web, history, bookmarks, section index and
search-as-you-type. Colors come from your terminal's own 16-color palette, so
it follows gruvbox/nord/whatever you run. Pages drawn by JavaScript are
rendered with an installed headless Chromium.

One static Go binary (~14 MB), MIT: https://github.com/sazardev/wr
Docs + demo: https://sazardev.github.io/wr/
Install: curl -fsSL https://raw.githubusercontent.com/sazardev/wr/main/install.sh | sh
```

### Otras comunidades

- **Lobsters** — https://lobste.rs/stories/new (necesitas invitación; tags `go`, `show`, `cli`)
- **Tildes** — https://tildes.net/~comp
- **dev.to** — https://dev.to/new : artículo «I built a terminal reader mode that keeps your code highlighted» con enlace al repo y al sitio. Cross-post a Hashnode (https://hashnode.com) y Medium.
- **Mastodon** — hashtags `#golang #terminal #TUI #CLI #OpenSource #BubbleTea #programming` con el enlace y el GIF.
- **Bluesky** — mismo texto; busca cuentas de Go/terminal y usa `#golang`.
- **Discord de Charm** — https://charm.sh/chat canal de showcase (wr está hecho con Bubble Tea).
- **Slack de Gophers** — https://invite.slack.golangbridge.org canal `#showandtell`.
- **Foro de Go** — https://forum.golangbridge.org (categoría *Show & Tell*).
- **Hacker News, segunda ronda**: publica un post técnico en el blog (extracción de artículos, detección de lenguaje) y enlázalo; no repitas el Show HN.

## 3. Listas «awesome» (PRs de una línea)

Cada lista tiene su CONTRIBUTING: respeta el orden alfabético y el formato
exacto. Entrada lista para copiar:

```
- [wr](https://github.com/sazardev/wr) - Reader mode and browser for the terminal: formatted Markdown, highlighted code, links, history, bookmarks and search.
```

- **awesome-tuis** — https://github.com/rothgar/awesome-tuis → sección **Web**
  (junto a browsh, carbonyl, Chawan, lynx). Encaja perfecto.
- **awesome-go** — https://github.com/avelino/awesome-go → revisa su
  CONTRIBUTING: la lista acepta sobre todo librerías; si aceptan la app, va en
  **Command Line**. Si la rechazan, no insistas.
- **Awesome-Linux-Software** — https://github.com/luong-komorebi/Awesome-Linux-Software → **Command Line**
- **awesome-mac** — https://github.com/jaywcjlove/awesome-mac → **Command Line**
- **awesome-cli-apps** — https://github.com/agarrharr/awesome-cli-apps → **Utilities** (opcional, encaja peor)
- **awesome-golang** alternativas: busca `topic:awesome-go` en GitHub si quieres más.

PR sugerido: título `Add wr — terminal reader mode and browser`, cuerpo con una
frase y el enlace a las capturas. Un PR por lista, no uno conjunto.

## 4. Newsletters y directorios

- **Terminal Trove** — https://terminaltrove.com/submit/ (formulario; es el
  directorio de referencia de TUIs)
- **Console** — https://console.dev/tools/ (submit; herramientas para devs)
- **Golang Weekly** — https://golangweekly.com (enlace «Submit a link» al pie)
- **Hacker Newsletter** — se nutre de HN; con un buen Show HN entras solo.
- **AlternativeTo** — https://alternativeto.net/manage/add-app/ → añade `wr`
  como alternativa a `lynx`, `w3m`, `browsh`, `readability-cli`.
- **LibHunt (Go)** — https://www.libhunt.com (se alimenta de GitHub; reclama la ficha)
- **Open Source Agenda** y **GitHub Explore**: automáticos si el repo gana estrellas y topics correctos.
- **StackShare** — https://stackshare.io (ficha de herramienta, opcional)

## 5. Gestores de paquetes

- **Homebrew (tap propio)** — crear `sazardev/homebrew-tap` con
  `Formula/wr.rb` que descargue el tarball del release y verifique el SHA-256.
  Queda: `brew install sazardev/tap/wr`. (Puedo generarlo; dime y lo hago.)
- **AUR (Arch)** — paquete `wr-bin` con un PKGBUILD que use
  `wr_linux_amd64.tar.gz` del release. Requiere tu cuenta AUR y clave SSH.
- **Nixpkgs** — PR a https://github.com/NixOS/nixpkgs con el paquete
  (buildGoModule). Más trabajo, muy buena distribución.
- **Scoop / Winget** — no hay binarios de Windows (se usa WSL); sáltalos hasta
  que exista un build nativo.
- **pkg.go.dev** — ya indexado; puedes añadir el badge al README:
  `[![Go Reference](https://pkg.go.dev/badge/github.com/sazardev/wr.svg)](https://pkg.go.dev/github.com/sazardev/wr)`

## 6. Medición

- Search Console → Rendimiento (consultas, impresiones, clics).
- Bing Webmaster → mismo panel.
- GitHub: `gh api repos/sazardev/wr/traffic/views` y
  `gh api repos/sazardev/wr/traffic/popular/referrers` (últimos 14 días).
- Estrellas y referrers tras cada post: `gh repo view sazardev/wr`.

## Orden recomendado

1. Search Console + Bing (hoy; sin esto, nada se indexa bien).
2. Terminal Trove y Console (formularios, sin riesgo).
3. PR a awesome-tuis (encaje perfecto) y Awesome-Linux-Software.
4. Show HN un martes–jueves por la mañana; Reddit ese mismo día.
5. dev.to + Mastodon/Bluesky la semana siguiente, con el GIF.
6. Homebrew tap y AUR cuando haya un par de versiones más.
