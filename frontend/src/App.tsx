import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import { Box, Container, CssBaseline, ThemeProvider, Typography } from '@mui/material';
import theme from './theme';
import AuthProvider from './auth/AuthProvider';
import NotificationProvider from './notifications/NotificationProvider';
import Header from './components/Header';
import PostList from './pages/PostList';
import PostDetail from './pages/PostDetail';
import AuthPage from './pages/AuthPage';
import ComposePost from './pages/ComposePost';
import NotFound from './pages/NotFound';

const App = () => {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Router>
        <NotificationProvider>
          <AuthProvider>
            <Container maxWidth="md">
              <Header />
              <Box component="main">
                <Routes>
                  <Route path="/" element={<PostList />} />
                  <Route path="/post/:id" element={<PostDetail />} />
                  <Route path="/login" element={<AuthPage type="login" />} />
                  <Route path="/signup" element={<AuthPage type="signup" />} />
                  <Route path="/admin/compose" element={<ComposePost />} />
                  <Route path="*" element={<NotFound />} />
                </Routes>
              </Box>
              <Box component="footer" sx={{ mt: 10, mb: 4, textAlign: 'center' }}>
                <Typography variant="caption" color="text.secondary">
                  &copy; {new Date().getFullYear()} Lumina. Built for stories.
                </Typography>
              </Box>
            </Container>
          </AuthProvider>
        </NotificationProvider>
      </Router>
    </ThemeProvider>
  );
};

export default App;
