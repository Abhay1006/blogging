import { describe, expect, it } from 'vitest';
import { stripMarkdown } from './stripMarkdown';

describe('stripMarkdown', () => {
  it('removes markdown syntax but keeps the text', () => {
    const input = '# Title\n\nSome **bold** and _italic_ text with a [link](https://x.com).\n\n> quoted';
    expect(stripMarkdown(input)).toBe('Title Some bold and italic text with a link. quoted');
  });

  it('drops block math and images, keeps inline math content', () => {
    expect(stripMarkdown('Area $\\pi r^2$ here\n$$\nE = mc^2\n$$\n![alt](img.png) end')).toBe('Area pi r^2 here end');
  });

  it('removes HTML tags', () => {
    expect(stripMarkdown('<b>Hi</b> <img src="x">there<script>alert(1)</script>')).toBe('Hi there');
  });
});
