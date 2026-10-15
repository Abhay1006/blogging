import { Link } from 'react-router-dom';
import { Box, Button, Typography } from '@mui/material';

const NotFound = () => (
  <Box sx={{ mt: 10, textAlign: 'center' }}>
    <Typography variant="h1" sx={{ mb: 2 }}>Page not found</Typography>
    <Typography color="text.secondary" sx={{ mb: 4 }}>
      The page you're looking for doesn't exist or has been moved.
    </Typography>
    <Button component={Link} to="/" variant="contained">Back to stories</Button>
  </Box>
);

export default NotFound;
