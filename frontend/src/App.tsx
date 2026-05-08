import { useState } from 'react';
import { 
  BrowserRouter as Router, 
  Routes, 
  Route, 
  Link, 
  useParams, 
  useNavigate 
} from 'react-router-dom';
import { 
  Container, 
  Typography, 
  Box, 
  TextField, 
  Button, 
  ThemeProvider, 
  CssBaseline,
  Link as MuiLink,
  Paper,
  Avatar,
  Stack
} from '@mui/material';
import { ThumbUp, ThumbDown, Edit as EditIcon } from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import axios from 'axios';
import { Formik, Form } from 'formik';
import theme from './theme';
import BlogRenderer from './components/BlogRenderer';

const API_URL = import.meta.env.VITE_API_URL;
console.log("Connecting to API at:", API_URL);

interface User {
  id: string;
  name: string;
  email: string;
  role: string;
}

interface Blog {
  id: string;
  title: string;
  body: string;
  author: string;
  likes: string[];
  dislikes: string[];
  created_at: string;
}

interface Comment {
  id: string;
  blog_id: string;
  user_id: string;
  user_name: string;
  content: string;
  created_at: string;
}

// --- Header ---

const Header = () => {
  const navigate = useNavigate();
  const userJson = localStorage.getItem('user');
  const user: User | null = userJson ? JSON.parse(userJson) : null;

  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    navigate('/login');
  };

  return (
    <Box sx={{ py: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <Typography variant="h4" sx={{ fontWeight: 700 }}>
        <MuiLink component={Link} to="/" sx={{ color: 'inherit', textDecoration: 'none' }}>
          Lumina
        </MuiLink>
      </Typography>
      <Box sx={{ display: 'flex', gap: 3, alignItems: 'center' }}>
        <MuiLink component={Link} to="/" sx={{ color: 'text.primary', fontWeight: 500 }}>Discover</MuiLink>
        {user?.role === 'admin' && (
          <MuiLink component={Link} to="/admin/compose" sx={{ color: 'text.primary', fontWeight: 500 }}>Compose</MuiLink>
        )}
        {user ? (
          <Button variant="outlined" size="small" onClick={handleLogout}>Log out</Button>
        ) : (
          <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
            <MuiLink component={Link} to="/login" sx={{ color: 'text.primary', fontWeight: 500 }}>Login</MuiLink>
            <Button component={Link} to="/signup" variant="contained" size="small">Sign up</Button>
          </Stack>
        )}
      </Box>
    </Box>
  );
};

// --- Pages ---

const PostList = () => {
  const { data: blogs, isLoading } = useQuery<Blog[]>({
    queryKey: ['blogs'],
    queryFn: async () => {
      const res = await axios.get(`${API_URL}/blogs`);
      return res.data;
    },
  });

  if (isLoading) return <Typography sx={{ mt: 4 }}>Loading...</Typography>;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 4, mt: 2 }}>
      {blogs?.map((blog) => (
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
              cursor: 'pointer'
            } 
          }}
        >
          <Typography variant="h2" sx={{ mb: 1 }}>{blog.title}</Typography>
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 2 }}>
            {new Date(blog.created_at).toLocaleDateString()} — {blog.author}
          </Typography>
          <Typography variant="body1" sx={{ 
            overflow: 'hidden', 
            textOverflow: 'ellipsis', 
            display: '-webkit-box', 
            WebkitLineClamp: 3, 
            WebkitBoxOrient: 'vertical',
            color: 'text.secondary',
            mb: 2
          }}>
            {blog.body.substring(0, 300)}...
          </Typography>
          <Stack direction="row" spacing={3} sx={{ color: 'text.secondary' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <ThumbUp sx={{ fontSize: '1.1rem' }} />
              <Typography variant="caption" sx={{ fontWeight: 600 }}>{blog.likes.length}</Typography>
            </Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <ThumbDown sx={{ fontSize: '1.1rem' }} />
              <Typography variant="caption" sx={{ fontWeight: 600 }}>{blog.dislikes.length}</Typography>
            </Box>
          </Stack>
        </Paper>
      ))}
    </Box>
  );
};

const PostDetail = () => {
  const { id } = useParams();
  const queryClient = useQueryClient();
  const token = localStorage.getItem('token');
  const userJson = localStorage.getItem('user');
  const currentUser: User | null = userJson ? JSON.parse(userJson) : null;

  const [isEditing, setIsEditing] = useState(false);
  const { data: blog, isLoading } = useQuery<Blog>({
    queryKey: ['blog', id],
    queryFn: async () => {
      const res = await axios.get(`${API_URL}/blogs/${id}`);
      return res.data;
    },
  });

  const { data: comments } = useQuery<Comment[]>({
    queryKey: ['comments', id],
    queryFn: async () => {
      const res = await axios.get(`${API_URL}/blogs/${id}/comments`);
      return res.data;
    },
  });

  const interactionMutation = useMutation({
    mutationFn: async (type: 'like' | 'dislike') => {
      await axios.post(`${API_URL}/blogs/${id}/${type}`, {}, {
        headers: { Authorization: `Bearer ${token}` }
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['blog', id] });
    },
    onError: (error: any) => {
      if (error.response?.status === 401) {
        alert('Session expired. Please login again.');
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        window.location.href = '/login';
      } else {
        alert('Failed to update interaction.');
      }
    }
  });

  const commentMutation = useMutation({
    mutationFn: async (content: string) => {
      await axios.post(`${API_URL}/blogs/${id}/comments`, { content }, {
        headers: { Authorization: `Bearer ${token}` }
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['comments', id] });
    },
    onError: (error: any) => {
      if (error.response?.status === 401) {
        alert('Session expired. Please login again.');
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        window.location.href = '/login';
      } else {
        alert('Failed to post comment.');
      }
    }
  });

  const editMutation = useMutation({
    mutationFn: async (values: { title: string; body: string }) => {
      await axios.put(`${API_URL}/blogs/${id}`, values, {
        headers: { Authorization: `Bearer ${token}` }
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['blog', id] });
      setIsEditing(false);
    },
  });

  if (isLoading) return <Typography>Loading...</Typography>;
  if (!blog) return <Typography>Post not found.</Typography>;

  if (isEditing) {
    return (
      <Box sx={{ mt: 4 }}>
        <Typography variant="h1" sx={{ mb: 4 }}>Edit Post</Typography>
        <Formik
          initialValues={{ title: blog.title, body: blog.body }}
          onSubmit={(values) => editMutation.mutate(values)}
        >
          {({ handleChange, values }) => (
            <Form>
              <TextField
                fullWidth
                label="Title"
                name="title"
                value={values.title}
                onChange={handleChange}
                sx={{ mb: 3 }}
              />
              <TextField
                fullWidth
                multiline
                rows={15}
                label="Body (Markdown supported)"
                name="body"
                value={values.body}
                onChange={handleChange}
                sx={{ mb: 3 }}
              />
              <Box sx={{ display: 'flex', gap: 2 }}>
                <Button type="submit" variant="contained" disabled={editMutation.isPending}>
                  {editMutation.isPending ? 'Saving...' : 'Save Changes'}
                </Button>
                <Button onClick={() => setIsEditing(false)} variant="outlined">Cancel</Button>
              </Box>
            </Form>
          )}
        </Formik>
      </Box>
    );
  }

  return (
    <Box sx={{ mt: 4 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
        <Typography variant="h1">{blog.title}</Typography>
        {currentUser?.role === 'admin' && (
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
          variant={blog.likes.includes(currentUser?.id || '') ? 'contained' : 'outlined'}
          onClick={() => interactionMutation.mutate('like')}
          disabled={!currentUser || interactionMutation.isPending}
        >
          {blog.likes.length}
        </Button>
        <Button 
          startIcon={<ThumbDown />} 
          variant={blog.dislikes.includes(currentUser?.id || '') ? 'contained' : 'outlined'}
          color="inherit"
          onClick={() => interactionMutation.mutate('dislike')}
          disabled={!currentUser || interactionMutation.isPending}
        >
          {blog.dislikes.length}
        </Button>
      </Box>

      <Typography variant="h2" sx={{ mb: 3 }}>Comments</Typography>
      
      {currentUser ? (
        <Box sx={{ mb: 4 }}>
          <Formik
            initialValues={{ content: '' }}
            onSubmit={(values, { resetForm }) => {
              if (values.content.trim()) {
                commentMutation.mutate(values.content, { onSuccess: () => resetForm() });
              }
            }}
          >
            {({ values, handleChange }) => (
              <Form>
                <TextField 
                  fullWidth 
                  name="content" 
                  placeholder="Share your thoughts..." 
                  multiline 
                  rows={3} 
                  value={values.content} 
                  onChange={handleChange}
                  sx={{ mb: 2 }}
                />
                <Button type="submit" variant="contained" disabled={commentMutation.isPending}>
                  {commentMutation.isPending ? 'Posting...' : 'Post Comment'}
                </Button>
              </Form>
            )}
          </Formik>
        </Box>
      ) : (
        <Typography sx={{ mb: 4, color: 'text.secondary' }}>Please login to comment.</Typography>
      )}

      <Stack spacing={3} sx={{ mb: 8 }}>
        {comments?.map((c) => (
          <Box key={c.id}>
            <Stack direction="row" spacing={2} sx={{ alignItems: 'center', mb: 1 }}>
              <Avatar sx={{ width: 32, height: 32, fontSize: '0.8rem' }}>{c.user_name[0]}</Avatar>
              <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>{c.user_name}</Typography>
              <Typography variant="caption" color="text.secondary">{new Date(c.created_at).toLocaleDateString()}</Typography>
            </Stack>
            <Typography variant="body2" sx={{ pl: 6 }}>{c.content}</Typography>
          </Box>
        ))}
      </Stack>
    </Box>
  );
};

const AuthPage = ({ type }: { type: 'login' | 'signup' }) => {
  const navigate = useNavigate();
  const mutation = useMutation({
    mutationFn: async (values: any) => {
      const endpoint = type === 'login' ? '/login' : '/signup';
      const res = await axios.post(`${API_URL}${endpoint}`, values);
      return res.data;
    },
    onSuccess: (data) => {
      if (type === 'login') {
        localStorage.setItem('token', data.token);
        localStorage.setItem('user', JSON.stringify(data.user));
        navigate('/');
      } else {
        alert('Account created! Please login.');
        navigate('/login');
      }
    },
    onError: () => alert('Failed. Please check your credentials.'),
  });

  return (
    <Container maxWidth="xs" sx={{ mt: 10 }}>
      <Paper sx={{ p: 4, textAlign: 'center' }}>
        <Typography variant="h2" sx={{ mb: 4 }}>{type === 'login' ? 'Welcome back' : 'Create account'}</Typography>
        <Formik
          initialValues={{ email: '', password: '', name: '' }}
          onSubmit={(values) => mutation.mutate(values)}
        >
          {({ values, handleChange }) => (
            <Form>
              <Stack spacing={2}>
                {type === 'signup' && <TextField name="name" label="Full Name" fullWidth value={values.name} onChange={handleChange} />}
                <TextField name="email" label="Email" fullWidth value={values.email} onChange={handleChange} />
                <TextField name="password" label="Password" type="password" fullWidth value={values.password} onChange={handleChange} />
                <Button type="submit" variant="contained" fullWidth size="large" sx={{ py: 1.5 }}>
                  {type === 'login' ? 'Login' : 'Sign up'}
                </Button>
                <MuiLink component={Link} to={type === 'login' ? '/signup' : '/login'} sx={{ fontSize: '0.9rem' }}>
                  {type === 'login' ? "Don't have an account? Sign up" : "Already have an account? Login"}
                </MuiLink>
              </Stack>
            </Form>
          )}
        </Formik>
      </Paper>
    </Container>
  );
};

const ComposePost = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const token = localStorage.getItem('token');
  const userJson = localStorage.getItem('user');
  const user: User | null = userJson ? JSON.parse(userJson) : null;

  const mutation = useMutation({
    mutationFn: (newBlog: Partial<Blog>) => {
      return axios.post(`${API_URL}/blogs`, newBlog, {
        headers: { Authorization: `Bearer ${token}` }
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['blogs'] });
      navigate('/');
    },
  });

  if (user?.role !== 'admin') {
    return <Typography sx={{ mt: 10, textAlign: 'center' }}>Admin access required.</Typography>;
  }

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h1" sx={{ mb: 4 }}>New Story</Typography>
      <Paper sx={{ p: 4 }}>
        <Formik
          initialValues={{ title: '', author: user.name, body: '' }}
          onSubmit={(values) => mutation.mutate(values)}
        >
          {({ values, handleChange }) => (
            <Form>
              <Stack spacing={3}>
                <TextField name="title" placeholder="Title" fullWidth variant="standard" slotProps={{ input: { style: { fontSize: '2.5rem', fontWeight: 700 } } }} value={values.title} onChange={handleChange} />
                <TextField name="body" placeholder="Tell your story..." multiline rows={15} fullWidth variant="standard" slotProps={{ input: { style: { fontSize: '1.2rem', lineHeight: 1.6 } } }} value={values.body} onChange={handleChange} />
                <Button type="submit" variant="contained" size="large" disabled={mutation.isPending}>Publish</Button>
              </Stack>
            </Form>
          )}
        </Formik>
      </Paper>
    </Box>
  );
};

const App = () => {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Router>
        <Container maxWidth="md">
          <Header />
          <Routes>
            <Route path="/" element={<PostList />} />
            <Route path="/post/:id" element={<PostDetail />} />
            <Route path="/login" element={<AuthPage type="login" />} />
            <Route path="/signup" element={<AuthPage type="signup" />} />
            <Route path="/admin/compose" element={<ComposePost />} />
          </Routes>
          <Box sx={{ mt: 10, mb: 4, textAlign: 'center' }}>
            <Typography variant="caption" color="text.secondary">
              &copy; {new Date().getFullYear()} Lumina. Built for stories.
            </Typography>
          </Box>
        </Container>
      </Router>
    </ThemeProvider>
  );
};

export default App;
