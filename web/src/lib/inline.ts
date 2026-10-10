// Inline Markdown to HTML for short, trusted strings (a table cell of the
// README, for example): escape first, then turn `code` into <code>. Never
// feed it untrusted input.

export function inlineHtml(text: string): string {
  const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  return escaped.replace(/`([^`]*)`/g, '<code>$1</code>');
}
