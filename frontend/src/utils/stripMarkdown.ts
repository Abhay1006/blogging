/** Converts Markdown (with LaTeX) to plain text for post previews. */
export const stripMarkdown = (text: string): string => {
  return text
    // 1. LaTeX
    .replace(/\$\$[\s\S]*?\$\$/g, '')        // Remove block math entirely
    .replace(/\$([^$]+)\$/g, '$1')           // Keep inline math content, remove $

    // 2. Backslashes used as line breaks
    .replace(/\\/g, ' ')

    // 3. Markdown elements
    .replace(/<(script|style)[\s\S]*?<\/\1>/gi, '') // Script/style blocks
    .replace(/<[^>]+>/g, '')                 // HTML tags
    .replace(/#+\s+/g, '')                   // Headers
    .replace(/!\[[^\]]*\]\([^)]+\)/g, '')    // Images
    .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1') // Links (keep text)
    .replace(/[*_~`]{1,3}/g, '')             // Bold, Italic, Strike, Code
    .replace(/^\s*>\s+/gm, '')               // Blockquotes

    // 4. Whitespace
    .replace(/\s+/g, ' ')
    .trim();
};
