import { Box, Button, Stack, TextField } from '@mui/material';
import { Form, Formik, type FormikErrors } from 'formik';
import type { BlogInput } from '../api/endpoints';

const MAX_TITLE_LENGTH = 200;

const validate = (values: BlogInput): FormikErrors<BlogInput> => {
  const errors: FormikErrors<BlogInput> = {};
  const title = values.title.trim();
  if (!title) errors.title = 'Title is required';
  else if (title.length > MAX_TITLE_LENGTH) errors.title = `Title must be at most ${MAX_TITLE_LENGTH} characters`;
  if (!values.body.trim()) errors.body = 'Body is required';
  return errors;
};

interface PostFormProps {
  initialValues: BlogInput;
  submitLabel: string;
  isPending: boolean;
  onSubmit: (values: BlogInput) => void;
  onCancel?: () => void;
  /** "compose" renders large borderless fields; "edit" renders labelled outlined fields. */
  variant: 'compose' | 'edit';
}

const PostForm = ({ initialValues, submitLabel, isPending, onSubmit, onCancel, variant }: PostFormProps) => {
  const compose = variant === 'compose';

  return (
    <Formik initialValues={initialValues} validate={validate} onSubmit={(values) => onSubmit({ ...values, title: values.title.trim() })}>
      {({ values, errors, touched, handleChange, handleBlur }) => (
        <Form noValidate>
          <Stack spacing={3}>
            <TextField
              name="title"
              label={compose ? undefined : 'Title'}
              placeholder={compose ? 'Title' : undefined}
              variant={compose ? 'standard' : 'outlined'}
              fullWidth
              value={values.title}
              onChange={handleChange}
              onBlur={handleBlur}
              error={touched.title && Boolean(errors.title)}
              helperText={touched.title && errors.title}
              slotProps={compose ? { input: { style: { fontSize: '2.5rem', fontWeight: 700 } } } : undefined}
            />
            <TextField
              name="body"
              label={compose ? undefined : 'Body (Markdown supported)'}
              placeholder={compose ? 'Tell your story... (Markdown supported)' : undefined}
              variant={compose ? 'standard' : 'outlined'}
              multiline
              rows={15}
              fullWidth
              value={values.body}
              onChange={handleChange}
              onBlur={handleBlur}
              error={touched.body && Boolean(errors.body)}
              helperText={touched.body && errors.body}
              slotProps={compose ? { input: { style: { fontSize: '1.2rem', lineHeight: 1.6 } } } : undefined}
            />
            <Box sx={{ display: 'flex', gap: 2 }}>
              <Button type="submit" variant="contained" size={compose ? 'large' : 'medium'} disabled={isPending}>
                {isPending ? 'Saving...' : submitLabel}
              </Button>
              {onCancel && <Button onClick={onCancel} variant="outlined" disabled={isPending}>Cancel</Button>}
            </Box>
          </Stack>
        </Form>
      )}
    </Formik>
  );
};

export default PostForm;
