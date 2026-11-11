import { Link, useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { UserPlus } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/common/Card';
import { Button } from '@/components/common/Button';
import { Input, Label } from '@/components/common/Input';
import { PageMeta } from '@/components/common/PageMeta';
import { useRegisterMutation } from '@/features/user-settings/useAuth';
import { useToastStore } from '@/store/toastStore';
import { APIClientError } from '@/services/apiClient';

const registerSchema = z.object({
  name: z.string().trim().min(2, 'Name must be at least 2 characters').max(120),
  email: z.string().trim().email('Please enter a valid email address'),
  password: z.string().min(8, 'Password must be at least 8 characters').max(72),
});

type RegisterValues = z.infer<typeof registerSchema>;

export function RegisterPage() {
  const navigate = useNavigate();
  const registerMutation = useRegisterMutation();
  const pushToast = useToastStore((s) => s.push);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<RegisterValues>({
    resolver: zodResolver(registerSchema),
  });

  const onSubmit = async (values: RegisterValues) => {
    try {
      await registerMutation.mutateAsync(values);
      pushToast({ title: 'Account created', description: 'Welcome to MediaDock!', variant: 'success' });
      navigate('/downloader', { replace: true });
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not create account';
      pushToast({ title: 'Registration failed', description: msg, variant: 'error' });
    }
  };

  return (
    <>
      <PageMeta title="Create Account" description="Register for a MediaDock account." noIndex />
      <div className="mx-auto flex max-w-container items-center justify-center px-4 py-16 sm:px-6">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle className="text-2xl">Create your MediaDock account</CardTitle>
            <CardDescription>Start managing and converting your authorized media.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
              <div className="space-y-1.5">
                <Label htmlFor="reg-name">Full Name</Label>
                <Input
                  id="reg-name"
                  type="text"
                  autoComplete="name"
                  placeholder="Alex Rivera"
                  aria-invalid={Boolean(errors.name)}
                  {...register('name')}
                />
                {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="reg-email">Email</Label>
                <Input
                  id="reg-email"
                  type="email"
                  autoComplete="email"
                  placeholder="you@example.com"
                  aria-invalid={Boolean(errors.email)}
                  {...register('email')}
                />
                {errors.email && <p className="text-xs text-destructive">{errors.email.message}</p>}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="reg-password">Password</Label>
                <Input
                  id="reg-password"
                  type="password"
                  autoComplete="new-password"
                  placeholder="Minimum 8 characters"
                  aria-invalid={Boolean(errors.password)}
                  {...register('password')}
                />
                {errors.password && <p className="text-xs text-destructive">{errors.password.message}</p>}
              </div>

              {registerMutation.isError && (
                <p role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {registerMutation.error instanceof APIClientError
                    ? registerMutation.error.message
                    : 'Registration failed.'}
                </p>
              )}

              <Button type="submit" className="w-full" size="lg" isLoading={registerMutation.isPending}>
                <UserPlus className="h-4 w-4" aria-hidden="true" />
                Create Account
              </Button>
            </form>

            <p className="mt-5 text-center text-sm text-muted-foreground">
              Already have an account?{' '}
              <Link to="/login" className="font-medium text-cyanHighlight hover:underline">
                Sign in
              </Link>
            </p>
          </CardContent>
        </Card>
      </div>
    </>
  );
}

