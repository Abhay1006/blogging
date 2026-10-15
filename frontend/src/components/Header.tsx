import { Link, useNavigate } from 'react-router-dom';
import { Box, Button, Link as MuiLink, Stack, Typography } from '@mui/material';
import { useAuth } from '../auth/AuthContext';

const Header = () => {
  const navigate = useNavigate();
  const { user, isAdmin, logout } = useAuth();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <Box component="header" sx={{ py: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <Typography variant="h4" component="div" sx={{ fontWeight: 700 }}>
        <MuiLink component={Link} to="/" sx={{ color: 'inherit', textDecoration: 'none' }}>
          Lumina
        </MuiLink>
      </Typography>
      <Box component="nav" sx={{ display: 'flex', gap: 3, alignItems: 'center' }}>
        <MuiLink component={Link} to="/" sx={{ color: 'text.primary', fontWeight: 500 }}>Discover</MuiLink>
        {isAdmin && (
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

export default Header;
