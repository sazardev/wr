// A tiny Markdown renderer, just enough for docs/GUIDE.md: headings, code
// fences, tables, lists, quotes, links, emphasis and inline code. No
// dependencies, no HTML injection (everything is escaped first).
(() => {
  'use strict';

  const ESCAPES = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
  const escapeHtml = (s) => s.replace(/[&<>"']/g, (c) => ESCAPES[c]);

  // .md links inside the repo point at the GitHub view of the file.
  function fixHref(href) {
    if (/^(https?:|#|mailto:)/.test(href)) return href;
    const m = href.match(/^(.*?)\.md(#.*)?$/);
    if (!m) return href;
    const path = m[1].replace(/^\.\.\//, '').replace(/^\.\//, '');
    return 'https://github.com/sazardev/wr/blob/main/' + path + '.md' + (m[2] || '');
  }

  function inline(src) {
    let s = escapeHtml(src);
    const codes = [];
    s = s.replace(/`([^`]+)`/g, (m, c) => {
      codes.push(c);
      return '\u0000' + (codes.length - 1) + '\u0000';
    });
    s = s.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (m, t, u) => '<a href="' + fixHref(u) + '">' + t + '</a>');
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/(^|[\s(])\*([^*\n]+)\*/g, '$1<em>$2</em>');
    s = s.replace(/(^|[^\w])_([^_\n]+)_(?=[^\w]|$)/g, '$1<em>$2</em>');
    s = s.replace(/\u0000(\d+)\u0000/g, (m, i) => '<code>' + codes[+i] + '</code>');
    return s;
  }

  const slug = (s) => s.toLowerCase().replace(/`/g, '').replace(/[^\w\s-]/g, '').trim().replace(/\s+/g, '-');
  const splitRow = (line) => line.trim().replace(/^\||\|$/g, '').split('|').map((c) => c.trim());

  function renderMarkdown(md) {
    const lines = md.split('\n');
    const lists = [];
    let html = '';
    let i = 0;

    const closeLists = (depth) => {
      while (lists.length && lists[lists.length - 1].indent >= depth) {
        html += lists.pop().type === 'ol' ? '</ol>' : '</ul>';
      }
    };

    while (i < lines.length) {
      const line = lines[i];

      if (!line.trim()) { closeLists(0); i++; continue; }

      let m = line.match(/^```(\S*)\s*$/);
      if (m) {
        const lang = m[1];
        const code = [];
        i++;
        while (i < lines.length && !/^```/.test(lines[i])) { code.push(lines[i]); i++; }
        i++;
        html += '<pre class="code"' + (lang ? ' data-lang="' + escapeHtml(lang) + '"' : '') + '><code>' + escapeHtml(code.join('\n')) + '</code></pre>';
        continue;
      }

      m = line.match(/^(#{1,6})\s+(.*)$/);
      if (m) {
        closeLists(0);
        const level = m[1].length;
        const text = m[2].replace(/\s+#+\s*$/, '');
        html += '<h' + level + ' id="' + slug(text) + '">' + inline(text) + '</h' + level + '>';
        i++;
        continue;
      }

      if (/^\s*(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
        closeLists(0);
        html += '<hr>';
        i++;
        continue;
      }

      if (line.trim().startsWith('|') && i + 1 < lines.length) {
        const sep = lines[i + 1];
        if (sep.includes('-') && /^\s*\|?[\s:|-]+\|?\s*$/.test(sep)) {
          closeLists(0);
          const head = splitRow(line);
          i += 2;
          const rows = [];
          while (i < lines.length && lines[i].trim().startsWith('|')) { rows.push(splitRow(lines[i])); i++; }
          html += '<div class="table-wrap"><table><thead><tr>' +
            head.map((c) => '<th>' + inline(c) + '</th>').join('') +
            '</tr></thead><tbody>' +
            rows.map((r) => '<tr>' + r.map((c) => '<td>' + inline(c) + '</td>').join('') + '</tr>').join('') +
            '</tbody></table></div>';
          continue;
        }
      }

      if (/^>\s?/.test(line)) {
        const buf = [];
        while (i < lines.length && /^>\s?/.test(lines[i])) {
          buf.push(lines[i].replace(/^>\s?/, ''));
          i++;
        }
        html += '<blockquote><p>' + inline(buf.join(' ')) + '</p></blockquote>';
        continue;
      }

      m = line.match(/^(\s*)([-*]|\d+\.)\s+(.*)$/);
      if (m) {
        const indent = m[1].length;
        const type = m[2].endsWith('.') ? 'ol' : 'ul';
        const text = [m[3]];
        i++;
        while (i < lines.length && /^\s{2,}\S/.test(lines[i]) && !/^\s*([-*]|\d+\.)\s/.test(lines[i])) {
          text.push(lines[i].trim());
          i++;
        }
        while (lists.length && lists[lists.length - 1].indent > indent) closeLists(lists[lists.length - 1].indent);
        const top = lists[lists.length - 1];
        if (top && top.indent === indent && top.type !== type) closeLists(indent);
        const now = lists[lists.length - 1];
        if (!now || now.indent < indent) {
          html += type === 'ol' ? '<ol>' : '<ul>';
          lists.push({ type, indent });
        }
        html += '<li>' + inline(text.join(' ')) + '</li>';
        continue;
      }

      const buf = [line.trim()];
      i++;
      while (i < lines.length && lines[i].trim() &&
        !/^(#{1,6}\s|```|>|\s*[-*]\s|\s*\d+\.\s|\||\s*(-{3,}|\*{3,})\s*$)/.test(lines[i])) {
        buf.push(lines[i].trim());
        i++;
      }
      closeLists(0);
      html += '<p>' + inline(buf.join(' ')) + '</p>';
    }
    closeLists(0);
    return html;
  }

  function buildTOC(doc, nav) {
    const heads = doc.querySelectorAll('h2, h3');
    nav.textContent = '';
    heads.forEach((h) => {
      const a = document.createElement('a');
      a.href = '#' + h.id;
      a.textContent = h.textContent;
      if (h.tagName === 'H3') a.className = 'sub';
      a.addEventListener('click', (e) => {
        e.preventDefault();
        h.scrollIntoView({ behavior: 'smooth', block: 'start' });
        history.replaceState(null, '', '#' + h.id);
      });
      nav.append(a);
    });
    if (!heads.length) return;
    const io = new IntersectionObserver((entries) => {
      entries.forEach((en) => {
        if (!en.isIntersecting) return;
        nav.querySelectorAll('a').forEach((a) => {
          a.classList.toggle('active', a.getAttribute('href') === '#' + en.target.id);
        });
      });
    }, { rootMargin: '-15% 0px -75% 0px' });
    heads.forEach((h) => io.observe(h));
  }

  window.renderMarkdown = renderMarkdown;
  window.buildTOC = buildTOC;
})();
