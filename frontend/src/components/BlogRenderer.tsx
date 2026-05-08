import ReactMarkdown from 'react-markdown';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import rehypeRaw from 'rehype-raw';
import 'katex/dist/katex.min.css';
import { Box } from '@mui/material';

interface BlogRendererProps {
  content: string;
}

const BlogRenderer: React.FC<BlogRendererProps> = ({ content }) => {
  return (
    <Box className="blog-content" sx={{ '& img': { maxWidth: '100%' } }}>
      <ReactMarkdown
        remarkPlugins={[remarkMath]}
        rehypePlugins={[rehypeRaw, rehypeKatex]}
      >
        {content}
      </ReactMarkdown>
    </Box>
  );
};

export default BlogRenderer;
