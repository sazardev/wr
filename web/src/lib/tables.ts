// The reference pages are generated, not written: the settings and paths
// tables of the README are parsed here and rendered as documentation, so the
// two can never disagree.

export interface Table {
  headers: string[];
  rows: string[][];
}

const isRow = (line: string) => line.trim().startsWith('|');
const isSeparator = (line: string) => /^\s*\|?[\s:|-]+\|[\s:|-]*$/.test(line.trim()) && line.includes('-');

/** Splits a Markdown table row into its cells. */
export function splitRow(line: string): string[] {
  const trimmed = line.trim().replace(/^\|/, '').replace(/\|$/, '');
  return trimmed.split('|').map((cell) => cell.trim());
}

/** Every Markdown table in a document, in order. */
export function parseTables(markdown: string): Table[] {
  const lines = markdown.split('\n');
  const tables: Table[] = [];
  let i = 0;
  while (i < lines.length) {
    if (!isRow(lines[i]!) || !isSeparator(lines[i + 1] ?? '')) {
      i++;
      continue;
    }
    const headers = splitRow(lines[i]!);
    const rows: string[][] = [];
    i += 2;
    while (i < lines.length && isRow(lines[i]!) && !isSeparator(lines[i]!)) {
      rows.push(splitRow(lines[i]!));
      i++;
    }
    tables.push({ headers, rows });
  }
  return tables;
}

/** The first table that comes after the given heading (## or ###). */
export function tableUnderHeading(markdown: string, heading: string): Table | undefined {
  const lines = markdown.split('\n');
  const wanted = new RegExp(`^#{2,3}\\s+${escapeRegExp(heading)}\\s*$`);
  const anyHeading = /^#{1,6}\s/;
  for (let i = 0; i < lines.length; i++) {
    if (!wanted.test(lines[i]!)) continue;
    for (let j = i + 1; j < lines.length; j++) {
      if (anyHeading.test(lines[j]!)) break;
      if (isRow(lines[j]!) && isSeparator(lines[j + 1] ?? '')) {
        return parseTables(lines.slice(j).join('\n'))[0];
      }
    }
    break;
  }
  return undefined;
}

/** Cell text without the Markdown syntax (`\`width\`` -> `width`). */
export function cellText(cell: string): string {
  return cell
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\*\*([^*]*)\*\*/g, '$1')
    .trim();
}

function escapeRegExp(source: string): string {
  return source.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
