import ReactMarkdown from 'react-markdown';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import rehypeRaw from 'rehype-raw';
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize';
import 'katex/dist/katex.min.css';
import { Box } from '@mui/material';

// Raw HTML in posts is allowed but sanitized (no scripts, event handlers or
// javascript: URLs). The math classes that remark-math emits are kept so that
// rehype-katex, which runs after sanitizing, can still find and render them.
const sanitizeSchema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    code: [['className', /^language-./, 'math-inline', 'math-display']],
  },
};

interface BlogRendererProps {
  content: string;
}

const BlogRenderer = ({ content }: BlogRendererProps) => {
  return (
    <Box className="blog-content" sx={{ '& img': { maxWidth: '100%' } }}>
      <ReactMarkdown
        remarkPlugins={[remarkMath]}
        rehypePlugins={[rehypeRaw, [rehypeSanitize, sanitizeSchema], rehypeKatex]}
      >
        {content}
      </ReactMarkdown>
    </Box>
  );
};

export default BlogRenderer;
