import { describe, expect, it } from 'vitest';
import { cellText, parseTables, tableUnderHeading } from './tables';

const readme = `## Configuration

Use **Settings** in the program.

| Key | Default | What it does |
|---|---|---|
| \`width\` | \`100\` | max width of the reading column (40–400); \`0\` = full width |
| \`links\` | \`"footnotes"\` | \`footnotes\`, \`inline\` or \`hidden\` |

### Where things live

| Path | What |
|---|---|
| \`~/.config/wr/config.toml\` | settings and key bindings |
`;

describe('parseTables', () => {
  const tables = parseTables(readme);

  it('finds every table', () => {
    expect(tables).toHaveLength(2);
  });

  it('reads headers and rows', () => {
    expect(tables[0]!.headers).toEqual(['Key', 'Default', 'What it does']);
    expect(tables[0]!.rows).toHaveLength(2);
    expect(tables[0]!.rows[0]![0]).toBe('`width`');
  });

  it('handles escaped pipes and empty cells', () => {
    const t = parseTables('| a | b |\n|---|---|\n| x |  |')[0]!;
    expect(t.rows[0]).toEqual(['x', '']);
  });
});

describe('tableUnderHeading', () => {
  it('returns the table of the given heading only', () => {
    const table = tableUnderHeading(readme, 'Configuration')!;
    expect(table.headers).toEqual(['Key', 'Default', 'What it does']);
  });

  it('works for level-3 headings', () => {
    const table = tableUnderHeading(readme, 'Where things live')!;
    expect(table.headers).toEqual(['Path', 'What']);
  });

  it('returns undefined when the heading has no table', () => {
    expect(tableUnderHeading(readme, 'Install')).toBeUndefined();
  });
});

describe('cellText', () => {
  it('removes code and bold syntax', () => {
    expect(cellText('`width`')).toBe('width');
    expect(cellText('**bold** text')).toBe('bold text');
  });
});
