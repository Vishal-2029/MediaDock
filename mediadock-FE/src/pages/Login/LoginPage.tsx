import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { LogIn } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/common/Card';
import { Button } from '@/components/common/Button';
import { Input, Label } from '@/components/common/Input';
import { PageMeta } from '@/components/common/PageMeta';
import { useLoginMutation } from '@/features/user-settings/useAuth';
import { useToastStore } from '@/store/toastStore';
import { APIClientError } from '@/services/apiClient';

const loginSchema = z.object({
  email: z.string().trim().email('Please enter a valid email address'),
  password: z.string().min(1, 'Password is required'),
});

type LoginValues = z.infer<typeof loginSchema>;

export function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const redirect = searchParams.get('redirect') || '/downloader';
  const loginMutation = useLoginMutation();
  const pushToast = useToastStore((s) => s.push);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = async (values: LoginValues) => {
    try {
      await loginMutation.mutateAsync(values);
      pushToast({ title: 'Welcome back to MediaDock', variant: 'success' });
      navigate(redirect, { replace: true });
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not sign in';
      pushToast({ title: 'Authentication failed', description: msg, variant: 'error' });
    }
  };

  return (
    <>
      <PageMeta title="Login" description="Sign in to your MediaDock account." noIndex />
      <div className="mx-auto flex max-w-container items-center justify-center px-4 py-16 sm:px-6">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle className="text-2xl">Sign in to MediaDock</CardTitle>
            <CardDescription>Access your downloader, converter, and media library.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
              <div className="space-y-1.5">
                <Label htmlFor="login-email">Email</Label>
                <Input
                  id="login-email"
                  type="email"
                  autoComplete="email"
                  placeholder="you@example.com"
                  aria-invalid={Boolean(errors.email)}
                  {...register('email')}
                />
                {errors.email && <p className="text-xs text-destructive">{errors.email.message}</p>}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="login-password">Password</Label>
                <Input
                  id="login-password"
                  type="password"
                  autoComplete="current-password"
                  placeholder="••••••••"
                  aria-invalid={Boolean(errors.password)}
                  {...register('password')}
                />
                {errors.password && <p className="text-xs text-destructive">{errors.password.message}</p>}
              </div>

              {loginMutation.isError && (
                <p role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {loginMutation.error instanceof APIClientError
                    ? loginMutation.error.message
                    : 'Invalid email or password.'}
                </p>
              )}

              <Button type="submit" className="w-full" size="lg" isLoading={loginMutation.isPending}>
                <LogIn className="h-4 w-4" aria-hidden="true" />
                Sign In
              </Button>
            </form>

            <p className="mt-5 text-center text-sm text-muted-foreground">
              New to MediaDock?{' '}
              <Link to="/register" className="font-medium text-cyanHighlight hover:underline">
                Create an account
              </Link>
            </p>
          </CardContent>
        </Card>
      </div>
    </>
  );
}

