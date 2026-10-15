import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import BlogRenderer from './BlogRenderer';

const render = (content: string) => renderToStaticMarkup(<BlogRenderer content={content} />);

describe('BlogRenderer', () => {
  it('strips scripts, event handlers and javascript: URLs from raw HTML', () => {
    const html = render('<script>alert(1)</script><img src="x" onerror="alert(1)"><a href="javascript:alert(1)">x</a>');
    expect(html).not.toContain('<script');
    expect(html).not.toContain('onerror');
    expect(html).not.toContain('javascript:');
  });

  it('keeps safe HTML and markdown', () => {
    const html = render('**bold** <em>em</em>');
    expect(html).toContain('<strong>bold</strong>');
    expect(html).toContain('<em>em</em>');
  });

  it('still renders inline and block math with KaTeX', () => {
    const html = render('Inline $x^2$ and\n\n$$\ny = mx + b\n$$');
    expect(html).toContain('class="katex"');
    expect(html).toContain('katex-display');
  });
});
