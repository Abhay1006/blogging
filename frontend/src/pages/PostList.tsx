import { Link } from 'react-router-dom';
import { Box, Paper, Stack, Typography } from '@mui/material';
import { ThumbDown, ThumbUp } from '@mui/icons-material';
import { useQuery } from '@tanstack/react-query';
import { fetchBlogs } from '../api/endpoints';
import { stripMarkdown } from '../utils/stripMarkdown';

const PostList = () => {
  const { data: blogs, isLoading, isError } = useQuery({ queryKey: ['blogs'], queryFn: fetchBlogs });

  if (isLoading) return <Typography sx={{ mt: 4 }}>Loading...</Typography>;
  if (isError) return <Typography sx={{ mt: 4 }} color="error">Couldn't load stories. Please try again later.</Typography>;
  if (!blogs?.length) return <Typography sx={{ mt: 4 }} color="text.secondary">No stories yet.</Typography>;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 4, mt: 2 }}>
      {blogs.map((blog) => (
        <Paper
          key={blog.id}
          component={Link}
          to={`/post/${blog.id}`}
          sx={{
            p: 4,
            display: 'block',
            textDecoration: 'none',
            color: 'inherit',
            transition: '0.2s',
            '&:hover': {
              transform: 'translateY(-2px)',
              boxShadow: '0 8px 24px rgba(0,0,0,0.08)',
            },
          }}
        >
          <Typography variant="h2" sx={{ mb: 1 }}>{blog.title}</Typography>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 2 }}>
            {new Date(blog.created_at).toLocaleDateString()} — {blog.author}
          </Typography>
          {/* The line clamp truncates and adds the ellipsis. */}
          <Typography variant="body1" sx={{
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            display: '-webkit-box',
            WebkitLineClamp: 3,
            WebkitBoxOrient: 'vertical',
            color: 'text.secondary',
            mb: 2,
          }}>
            {stripMarkdown(blog.body)}
          </Typography>
          <Stack direction="row" spacing={3} sx={{ color: 'text.secondary' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }} aria-label={`${blog.like_count} likes`}>
              <ThumbUp sx={{ fontSize: '1.1rem' }} />
              <Typography variant="caption" sx={{ fontWeight: 600 }}>{blog.like_count}</Typography>
            </Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }} aria-label={`${blog.dislike_count} dislikes`}>
              <ThumbDown sx={{ fontSize: '1.1rem' }} />
              <Typography variant="caption" sx={{ fontWeight: 600 }}>{blog.dislike_count}</Typography>
            </Box>
          </Stack>
        </Paper>
      ))}
    </Box>
  );
};

export default PostList;
