import { Link, useNavigate } from 'react-router-dom';
import { Button, Container, Link as MuiLink, Paper, Stack, TextField, Typography } from '@mui/material';
import { useMutation } from '@tanstack/react-query';
import { Form, Formik, type FormikErrors } from 'formik';
import { login, signup, type SignupInput } from '../api/endpoints';
import { useAuth } from '../auth/AuthContext';
import { useNotify, useNotifyError } from '../notifications/NotificationContext';

type AuthType = 'login' | 'signup';

const MIN_PASSWORD_LENGTH = 8;
const MAX_PASSWORD_LENGTH = 72;
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const validate = (type: AuthType) => (values: SignupInput): FormikErrors<SignupInput> => {
  const errors: FormikErrors<SignupInput> = {};
  if (type === 'signup' && !values.name.trim()) errors.name = 'Name is required';
  if (!EMAIL_PATTERN.test(values.email.trim())) errors.email = 'Enter a valid email';
  if (!values.password) {
    errors.password = 'Password is required';
  } else if (type === 'signup' && (values.password.length < MIN_PASSWORD_LENGTH || values.password.length > MAX_PASSWORD_LENGTH)) {
    errors.password = `Password must be ${MIN_PASSWORD_LENGTH}–${MAX_PASSWORD_LENGTH} characters`;
  }
  return errors;
};

const AuthPage = ({ type }: { type: AuthType }) => {
  const navigate = useNavigate();
  const auth = useAuth();
  const notify = useNotify();
  const notifyError = useNotifyError();
  const isLogin = type === 'login';

  const mutation = useMutation({
    mutationFn: async (values: SignupInput) => {
      const email = values.email.trim();
      if (isLogin) {
        auth.login(await login({ email, password: values.password }));
      } else {
        await signup({ name: values.name.trim(), email, password: values.password });
      }
    },
    onSuccess: () => {
      if (isLogin) {
        navigate('/');
      } else {
        notify('Account created! Please log in.', 'success');
        navigate('/login');
      }
    },
    onError: (error) => notifyError(error, isLogin ? 'Login failed. Please try again.' : 'Sign up failed. Please try again.'),
  });

  return (
    <Container maxWidth="xs" sx={{ mt: 10 }}>
      <Paper sx={{ p: 4, textAlign: 'center' }}>
        <Typography variant="h2" component="h1" sx={{ mb: 4 }}>{isLogin ? 'Welcome back' : 'Create account'}</Typography>
        <Formik
          key={type}
          initialValues={{ email: '', password: '', name: '' }}
          validate={validate(type)}
          onSubmit={(values) => mutation.mutate(values)}
        >
          {({ values, errors, touched, handleChange, handleBlur }) => (
            <Form noValidate>
              <Stack spacing={2}>
                {!isLogin && (
                  <TextField
                    name="name" label="Full Name" autoComplete="name" fullWidth
                    value={values.name} onChange={handleChange} onBlur={handleBlur}
                    error={touched.name && Boolean(errors.name)} helperText={touched.name && errors.name}
                  />
                )}
                <TextField
                  name="email" label="Email" type="email" autoComplete="email" fullWidth
                  value={values.email} onChange={handleChange} onBlur={handleBlur}
                  error={touched.email && Boolean(errors.email)} helperText={touched.email && errors.email}
                />
                <TextField
                  name="password" label="Password" type="password" fullWidth
                  autoComplete={isLogin ? 'current-password' : 'new-password'}
                  value={values.password} onChange={handleChange} onBlur={handleBlur}
                  error={touched.password && Boolean(errors.password)} helperText={touched.password && errors.password}
                />
                <Button type="submit" variant="contained" fullWidth size="large" sx={{ py: 1.5 }} disabled={mutation.isPending}>
                  {mutation.isPending ? 'Please wait...' : isLogin ? 'Login' : 'Sign up'}
                </Button>
                <MuiLink component={Link} to={isLogin ? '/signup' : '/login'} sx={{ fontSize: '0.9rem' }}>
                  {isLogin ? "Don't have an account? Sign up" : 'Already have an account? Login'}
                </MuiLink>
              </Stack>
            </Form>
          )}
        </Formik>
      </Paper>
    </Container>
  );
};

export default AuthPage;
