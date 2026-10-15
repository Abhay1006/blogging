import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { Avatar, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material';
import { Edit as EditIcon, ThumbDown, ThumbUp } from '@mui/icons-material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Form, Formik } from 'formik';
import { fetchBlog, fetchComments, postComment, updateBlog, vote, type BlogInput } from '../api/endpoints';
import { isNotFound } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { useNotify, useNotifyError } from '../notifications/NotificationContext';
import BlogRenderer from '../components/BlogRenderer';
import PostForm from '../components/PostForm';
import NotFound from './NotFound';
import type { Blog } from '../types';

const MAX_COMMENT_LENGTH = 5000;

const PostDetail = () => {
  const { id = '' } = useParams();
  const queryClient = useQueryClient();
  const { user, isAdmin } = useAuth();
  const notify = useNotify();
  const notifyError = useNotifyError();
  const [isEditing, setIsEditing] = useState(false);

  const blogQuery = useQuery({ queryKey: ['blog', id], queryFn: () => fetchBlog(id) });
  const commentsQuery = useQuery({ queryKey: ['comments', id], queryFn: () => fetchComments(id) });

  const onBlogChanged = (blog: Blog) => {
    queryClient.setQueryData(['blog', id], blog);
    queryClient.invalidateQueries({ queryKey: ['blogs'] });
  };

  const voteMutation = useMutation({
    mutationFn: (type: 'like' | 'dislike') => vote(id, type),
    onSuccess: onBlogChanged,
    onError: (error) => notifyError(error, 'Failed to update your vote.'),
  });

  const commentMutation = useMutation({
    mutationFn: (content: string) => postComment(id, content),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['comments', id] }),
    onError: (error) => notifyError(error, 'Failed to post comment.'),
  });

  const editMutation = useMutation({
    mutationFn: (values: BlogInput) => updateBlog(id, values),
    onSuccess: (blog) => {
      onBlogChanged(blog);
      setIsEditing(false);
      notify('Post updated.', 'success');
    },
    onError: (error) => notifyError(error, 'Failed to save changes.'),
  });

  if (blogQuery.isLoading) return <Typography sx={{ mt: 4 }}>Loading...</Typography>;
  if (blogQuery.isError) {
    return isNotFound(blogQuery.error)
      ? <NotFound />
      : <Typography sx={{ mt: 4 }} color="error">Couldn't load this story. Please try again later.</Typography>;
  }
  const blog = blogQuery.data;
  if (!blog) return <NotFound />;

  if (isEditing) {
    return (
      <Box sx={{ mt: 4 }}>
        <Typography variant="h1" sx={{ mb: 4 }}>Edit Post</Typography>
        <PostForm
          variant="edit"
          initialValues={{ title: blog.title, body: blog.body }}
          submitLabel="Save Changes"
          isPending={editMutation.isPending}
          onSubmit={(values) => editMutation.mutate(values)}
          onCancel={() => setIsEditing(false)}
        />
      </Box>
    );
  }

  return (
    <Box component="article" sx={{ mt: 4 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
        <Typography variant="h1">{blog.title}</Typography>
        {isAdmin && (
          <Button variant="outlined" startIcon={<EditIcon />} onClick={() => setIsEditing(true)}>
            Edit
          </Button>
        )}
      </Box>
      <Typography variant="subtitle2" sx={{ mb: 4, fontSize: '1rem' }}>
        {new Date(blog.created_at).toLocaleDateString()} — {blog.author}
      </Typography>
      <Paper sx={{ p: 4, mb: 4 }}>
        <BlogRenderer content={blog.body} />
      </Paper>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 6 }}>
        <Button
          startIcon={<ThumbUp />}
          variant={blog.liked_by_me ? 'contained' : 'outlined'}
          onClick={() => voteMutation.mutate('like')}
          disabled={!user || voteMutation.isPending}
          aria-label="Like"
          aria-pressed={blog.liked_by_me}
        >
          {blog.like_count}
        </Button>
        <Button
          startIcon={<ThumbDown />}
          variant={blog.disliked_by_me ? 'contained' : 'outlined'}
          color="inherit"
          onClick={() => voteMutation.mutate('dislike')}
          disabled={!user || voteMutation.isPending}
          aria-label="Dislike"
          aria-pressed={blog.disliked_by_me}
        >
          {blog.dislike_count}
        </Button>
      </Box>

      <Typography variant="h2" sx={{ mb: 3 }}>Comments</Typography>

      {user ? (
        <Box sx={{ mb: 4 }}>
          <Formik
            initialValues={{ content: '' }}
            validate={({ content }) =>
              content.length > MAX_COMMENT_LENGTH ? { content: `Comments are limited to ${MAX_COMMENT_LENGTH} characters` } : {}}
            onSubmit={({ content }, { resetForm }) => {
              if (content.trim()) {
                commentMutation.mutate(content.trim(), { onSuccess: () => resetForm() });
              }
            }}
          >
            {({ values, errors, handleChange }) => (
              <Form>
                <TextField
                  fullWidth
                  name="content"
                  placeholder="Share your thoughts..."
                  multiline
                  rows={3}
                  value={values.content}
                  onChange={handleChange}
                  error={Boolean(errors.content)}
                  helperText={errors.content}
                  sx={{ mb: 2 }}
                />
                <Button type="submit" variant="contained" disabled={commentMutation.isPending || !values.content.trim()}>
                  {commentMutation.isPending ? 'Posting...' : 'Post Comment'}
                </Button>
              </Form>
            )}
          </Formik>
        </Box>
      ) : (
        <Typography sx={{ mb: 4, color: 'text.secondary' }}>Please login to comment.</Typography>
      )}

      {commentsQuery.isError && (
        <Typography color="error" sx={{ mb: 4 }}>Couldn't load comments.</Typography>
      )}
      <Stack spacing={3} sx={{ mb: 8 }}>
        {commentsQuery.data?.map((c) => (
          <Box key={c.id}>
            <Stack direction="row" spacing={2} sx={{ alignItems: 'center', mb: 1 }}>
              <Avatar sx={{ width: 32, height: 32, fontSize: '0.8rem' }}>{(c.user_name ?? '').charAt(0).toUpperCase()}</Avatar>
              <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>{c.user_name}</Typography>
              <Typography variant="caption" color="text.secondary">{new Date(c.created_at).toLocaleDateString()}</Typography>
            </Stack>
            <Typography variant="body2" sx={{ pl: 6, whiteSpace: 'pre-wrap' }}>{c.content}</Typography>
          </Box>
        ))}
      </Stack>
    </Box>
  );
};

export default PostDetail;
