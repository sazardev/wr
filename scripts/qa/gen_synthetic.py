#!/usr/bin/env python3
# Generates controlled synthetic cases to probe wr's extraction and rendering.
import os
from pathlib import Path

D = Path(os.environ.get("QA_ROOT", "qa-out")).resolve() / "synthetic"
D.mkdir(parents=True, exist_ok=True)

def w(name, html):
    (D / name).write_text(html, encoding="utf-8")

w("rich-article.html", """<!doctype html><html><head><title>Rich Article</title></head><body>
<nav>NAVJUNK: Home | About</nav>
<article>
<h1>The Title</h1>
<p>Intro with <strong>bold</strong>, <em>italic</em>, <code>inline code</code> and a <a href="/rel">relative link</a>.</p>
<h2>Lists</h2>
<ul><li>one<ul><li>nested a</li><li>nested b</li></ul></li><li>two</li></ul>
<ol><li>first</li><li>second<ol><li>deep</li></ol></li></ol>
<blockquote><p>A quote.</p><blockquote><p>Nested quote.</p></blockquote></blockquote>
<h3>Table</h3>
<table><thead><tr><th>Name</th><th>Value</th></tr></thead><tbody>
<tr><td>alpha</td><td>1</td></tr><tr><td>beta</td><td>2</td></tr></tbody></table>
<pre class="language-go"><code>func main() { fmt.Println("hi") }</code></pre>
<h4>End</h4><p>Final <a href="#lists">jump</a>.</p>
<img src="x.png" alt="A diagram">
<hr>
</article>
<footer>FOOTERJUNK</footer></body></html>""")

w("tables-complex.html", """<!doctype html><html><head><title>Tables</title></head><body><article>
<h1>Complex tables</h1>
<table>
<tr><th rowspan="2">Rowspan</th><th colspan="2">Colspan pair</th></tr>
<tr><td>a</td><td>b</td></tr>
</table>
<table><tr><td>no header</td><td>2nd</td></tr><tr><td></td><td>empty above</td></tr></table>
<table><tr><th>Item</th></tr><tr><td><ul><li>list in cell</li><li>two</li></ul></td></tr></table>
</article></body></html>""")

w("code-langs.html", """<!doctype html><html><head><title>Code</title></head><body><article>
<h1>Code blocks</h1>
<pre class="language-go"><code>package main
func main() { println("go") }</code></pre>
<pre data-language="python"><code>def f(x):
    return x + 1</code></pre>
<div class="highlight-source-rust"><pre><code>fn main() { println!("rust"); }</code></pre></div>
<pre><code>no language here
second line</code></pre>
<table class="highlight"><tr><td class="line-no">1</td><td class="code"><pre><code>line numbers wrapper</code></pre></td></tr></table>
<pre><code>fence test:
```
inner fence
```</code></pre>
<pre><code>def f():
    return "&lt;html&gt; &amp; entities"</code></pre>
</article></body></html>""")

w("semantics.html", """<!doctype html><html><head><title>Semantics</title></head><body><article>
<h1>Semantic tags</h1>
<details><summary>Click to expand</summary><p>Hidden text inside details.</p></details>
<figure><img src="f.png" alt="figure alt"><figcaption>The caption text.</figcaption></figure>
<dl><dt>Term</dt><dd>Definition of term</dd><dt>Other</dt><dd>Second definition</dd></dl>
<p><abbr title="HyperText Markup Language">HTML</abbr>, <kbd>Ctrl</kbd>+<kbd>C</kbd>, x<sup>2</sup>, H<sub>2</sub>O, <mark>marked</mark>, <del>gone</del>, <ins>new</ins>.</p>
<address>123 Street</address>
<time datetime="2024-01-01">Jan 1</time>
<video src="v.mp4"></video><audio src="a.mp3"></audio>
<form><label>Name <input type="text"></label><button>Send</button></form>
</article></body></html>""")

w("i18n.html", """<!doctype html><html><head><title>Internacional</title><meta charset="utf-8"></head><body><article>
<h1>I18N çontent</h1>
<p>Español: mañana será otro día, ¿verdad? ¡Sí! Accentuación: áéíóúüñç.</p>
<p>中文：这是一个测试段落，包含标点符号。日本語のテキストもあります。</p>
<p>العربية: هذا نص تجريبي طويل لاختبار اتجاه النص من اليمين إلى اليسار مع أرقام 12345.</p>
<p>עברית: זהו טקסט לבדיקת כיווניות.</p>
<p>Emoji: 🚀 🎉 👍🏽 test. Combining: e\u0301 a\u0300. Math: ∑ ∫ √ ≈ ≤ ≥.</p>
<p>Zero width: a\u200bb, non-breaking: 10&nbsp;000, soft hyphen: co&shy;operate.</p>
<p>Very long URL: https://example.com/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa?q=1</p>
</article></body></html>""")

w("layout-no-article.html", """<!doctype html><html><head><title>Layout Page</title></head><body>
<header><h1>Site Name</h1><nav>Menu One Two Three</nav></header>
<div class="breadcrumbs">Home &gt; Section &gt; Page</div>
<div class="toc"><ul><li><a href="#a">Section A</a></li><li><a href="#b">Section B</a></li></ul></div>
<main>
<h2 id="a">Section A</h2><p>Real content A with enough words to be meaningful in this test page.</p>
<h2 id="b">Section B</h2><p>Real content B with enough words to be meaningful in this test page.</p>
</main>
<aside class="sidebar"><h3>Related</h3><ul><li><a href="/x">X</a></li></ul></aside>
<footer><p>Copyright 2024</p></footer>
<script>var x=1;</script>
</body></html>""")

w("main-only.html", """<!doctype html><html><head><title>Main Only</title></head><body><main>
<h1>Main heading</h1><p>Only main content here, long enough to matter for the extraction heuristic test.</p>
</main></body></html>""")

w("body-only-noarticle-nomain.html", """<!doctype html><html><head><title>Body Only</title></head><body>
<h1>Body heading</h1><p>Body paragraph with content that has no article or main wrapper at all, this must still be extracted.</p>
</body></html>""")

w("junk-toc-aside.html", """<!doctype html><html><head><title>Junk</title></head><body><article>
<div class="cookie-banner">We use cookies. Accept all.</div>
<nav class="breadcrumbs">Home / Post</nav>
<aside class="sidebar"><ul><li><a href="/1">Sidebar link one</a></li><li><a href="/2">Sidebar link two</a></li></ul></aside>
<h1>Post</h1>
<div class="toc"><ol><li><a href="#s1">S1</a></li></ol></div>
<p>The actual post body text that we want to keep in the extraction result.</p>
<div class="newsletter-signup">Subscribe to our newsletter!</div>
<div class="related-posts"><h3>Related posts</h3><a href="/r1">Related one</a></div>
<div class="comments"><h3>Comments</h3><p>First comment.</p></div>
</article></body></html>""")

w("wikitable.html", """<!doctype html><html><head><title>Info</title></head><body><article>
<h1>Subject</h1>
<table class="infobox"><caption>Subject</caption>
<tr><th>Born</th><td>1 January 1900<br>Somewhere</td></tr>
<tr><th>Died</th><td>2 February 2000</td></tr>
<tr><th>Known for</th><td>Things</td></tr>
</table>
<p>Lead paragraph of the article with substantial content for the test.</p>
<h2>Life</h2><p>Biography text.</p>
</article></body></html>""")

w("headings-weird.html", """<!doctype html><html><head><title>Headings</title></head><body><article>
<h3>Starts at h3</h3><p>Text under h3.</p>
<h5>Jumps to h5</h5><p>Text under h5.</p>
<h2>Heading with <a href="/x">a link</a> and <code>code</code></h2><p>Text.</p>
<h2></h2><p>Empty heading above.</p>
<h2>   </h2><p>Whitespace heading above.</p>
</article></body></html>""")

w("links.html", """<!doctype html><html><head><title>Links</title></head><body><article>
<h1>Links</h1>
<p><a href="/relative/path?q=1#frag">Relative</a>,
<a href="#only-fragment">Fragment only</a>,
<a href="mailto:x@example.com">Mail</a>,
<a href="javascript:alert(1)">JS</a>,
<a href="//cdn.example.com/x">Protocol relative</a>,
<a href="ftp://files.example.com/f">FTP</a>,
<a href="tel:+341234567">Tel</a>,
<a href="https://other.example.com/page?utm_source=test">Absolute with utm</a>.</p>
<p>Bare text URL: https://bare.example.com/x not a link.</p>
<h2><a href="/inside-heading">Heading that is a link</a></h2>
<p><a href="">Empty href</a> and <a>No href at all</a>.</p>
</article></body></html>""")

w("images.html", """<!doctype html><html><head><title>Images</title></head><body><article>
<h1>Images</h1>
<img src="a.png" alt="Alt text here">
<img src="b.png" alt="">
<img src="c.png">
<picture><source srcset="d.webp" type="image/webp"><img src="d.png" alt="Picture alt"></picture>
<figure><img src="e.png" alt="Only image"><figcaption>Caption separate</figcaption></figure>
</article></body></html>""")

w("deep-nesting.html", """<!doctype html><html><head><title>Deep</title></head><body><article>
<h1>Deep nesting</h1>
<blockquote><p>L1</p><blockquote><p>L2</p><blockquote><p>L3</p><blockquote><p>L4</p><blockquote><p>L5</p></blockquote></blockquote></blockquote></blockquote></blockquote>
<ul><li>a<ul><li>b<ul><li>c<ul><li>d<ul><li>e<ul><li>f deep item</li></ul></li></ul></li></ul></li></ul></li></ul></li></ul>
<div><div><div><div><p>Deep div paragraph.</p></div></div></div></div>
</article></body></html>""")

w("entities.html", """<!doctype html><html><head><title>Entities &amp; more</title></head><body><article>
<h1>Entities</h1>
<p>&amp;amp; literal, &copy; &reg; &trade; &hellip; &mdash; &ndash; &laquo;quoted&raquo; &#8217;apostrophe&#8217; &euro;100 &pound;50 &frac12; &times; &divide; &plusmn; &infin; &ne; &le; &ge;.</p>
<p>&lt;script&gt;alert('not a real script')&lt;/script&gt; and &lt;b&gt;not bold&lt;/b&gt;.</p>
</article></body></html>""")

w("latin1.html", '<!doctype html><html><head><title>Latin1</title><meta http-equiv="Content-Type" content="text/html; charset=iso-8859-1"></head><body><article>\n<h1>Acentos en ISO-8859-1</h1><p>La ma\xf1ana de un d\xeda soleado, el ni\xf1o comi\xf3 jam\xf3n espa\xf1ol con mucha calma y diversi\xf3n.</p>\n</article></body></html>'.encode("latin-1").decode("latin-1"))

w("long-content.html", "<!doctype html><html><head><title>Long</title></head><body><article><h1>Long doc</h1>" + "".join(
    f"<h2>Section {i}</h2><p>Paragraph {i} with several words of content to give the renderer something to wrap around and measure. " * 3 + "</p>" for i in range(1, 201)) + "</article></body></html>")

w("md-weird.md", """# Markdown local file

Footnotes[^1] and escaped \\*asterisks\\* plus ~~strike~~.

[^1]: The footnote text.

<div>Raw HTML block</div>

| a | b |
|---|:-:|
| 1 | 2 |

Term
: definition list style

- [ ] task open
- [x] task done

    indented code block

> quote
>> deeper

Auto link <https://example.com> and bare https://example.com/x.
""")

w("plain.txt", "Plain text file line one\nLine two with <not html> but looks like it\n\nFinal line.\n")
w("data.json", '{"name": "wr", "version": "0.2.0", "tags": ["reader", "tui"], "nested": {"a": [1,2,3]}}\n')

print("synthetic files:", len(list(D.iterdir())))
