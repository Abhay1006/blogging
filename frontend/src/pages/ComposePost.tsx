import { useNavigate } from 'react-router-dom';
import { Paper, Typography } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createBlog } from '../api/endpoints';
import { useAuth } from '../auth/AuthContext';
import { useNotify, useNotifyError } from '../notifications/NotificationContext';
import PostForm from '../components/PostForm';

const ComposePost = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isAdmin } = useAuth();
  const notify = useNotify();
  const notifyError = useNotifyError();

  const mutation = useMutation({
    mutationFn: createBlog,
    onSuccess: (blog) => {
      queryClient.invalidateQueries({ queryKey: ['blogs'] });
      notify('Story published.', 'success');
      navigate(`/post/${blog.id}`);
    },
    onError: (error) => notifyError(error, 'Failed to publish the story.'),
  });

  if (!isAdmin) {
    return <Typography sx={{ mt: 10, textAlign: 'center' }}>Admin access required.</Typography>;
  }

  return (
    <>
      <Typography variant="h1" sx={{ mt: 4, mb: 4 }}>New Story</Typography>
      <Paper sx={{ p: 4 }}>
        <PostForm
          variant="compose"
          initialValues={{ title: '', body: '' }}
          submitLabel="Publish"
          isPending={mutation.isPending}
          onSubmit={(values) => mutation.mutate(values)}
        />
      </Paper>
    </>
  );
};

export default ComposePost;
