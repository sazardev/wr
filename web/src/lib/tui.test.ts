import { describe, expect, it } from 'vitest';
import { FILL, progressBar, progressCells, readPercent, scrollbarChars, TRACK } from './tui';

describe('progressBar', () => {
  it('fills whole and partial braille cells', () => {
    expect(progressBar(4, 50)).toEqual({ full: '⣿⣿', empty: '⣀⣀' });
    expect(progressBar(4, 0)).toEqual({ full: '', empty: '⣀⣀⣀⣀' });
    expect(progressBar(4, 100)).toEqual({ full: '⣿⣿⣿⣿', empty: '' });
    expect(progressBar(6, 25).full.length).toBeGreaterThan(1);
  });

  it('uses a plain fallback when braille is off', () => {
    expect(progressBar(4, 50, false)).toEqual({ full: '██', empty: '░░' });
  });

  it('clamps out-of-range percentages', () => {
    expect(progressBar(2, -10)).toEqual({ full: '', empty: '⣀⣀' });
    expect(progressBar(2, 500)).toEqual({ full: '⣿⣿', empty: '' });
  });
});

describe('scrollbarChars', () => {
  it('draws a track when the document fits', () => {
    expect(scrollbarChars(10, 10, 10, 0)).toEqual([]);
  });

  it('moves the thumb to the bottom at the end', () => {
    const top = scrollbarChars(10, 100, 10, 0);
    const bottom = scrollbarChars(10, 100, 10, 90);
    expect(top).toHaveLength(10);
    expect(top[0]).not.toBe(TRACK);
    expect(bottom[9]).not.toBe(TRACK);
    expect(bottom[0]).toBe(TRACK);
  });

  it('uses plain glyphs when braille is off', () => {
    const chars = scrollbarChars(5, 50, 10, 0, false);
    expect(chars).toHaveLength(5);
    expect(chars[0]).toBe('┃');
    expect(chars[4]).toBe('│');
  });
});

describe('readPercent', () => {
  it('is 100 when everything fits', () => {
    expect(readPercent(0, 0)).toBe(100);
  });
  it('rounds the reading position', () => {
    expect(readPercent(45, 90)).toBe(50);
    expect(readPercent(90, 90)).toBe(100);
  });
});

describe('progressCells', () => {
  it('adapts to the width', () => {
    expect(progressCells(120)).toBe(16);
    expect(progressCells(90)).toBe(12);
    expect(progressCells(70)).toBe(8);
    expect(progressCells(50)).toBe(5);
    expect(progressCells(30)).toBe(0);
  });
});

describe('FILL', () => {
  it('goes from empty to full', () => {
    expect(FILL[0]).toBe('⣀');
    expect(FILL[FILL.length - 1]).toBe('⣿');
  });
});
