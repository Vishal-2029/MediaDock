import { useEffect } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { authService, type LoginPayload, type RegisterPayload } from '@/services/authService';
import { settingsService, type UpdateSettingsPayload } from '@/services/settingsService';
import { useAuthStore } from '@/store/authStore';
import { useThemeStore } from '@/store/themeStore';

export function useSessionBootstrap() {
  const setUser = useAuthStore((s) => s.setUser);
  const clearAuth = useAuthStore((s) => s.clearAuth);

  const query = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: ({ signal }) => authService.me(signal),
    retry: false,
    staleTime: 5 * 60 * 1000,
  });

  useEffect(() => {
    if (query.isSuccess && query.data) {
      setUser(query.data);
    } else if (query.isError) {
      clearAuth();
    }
  }, [query.isSuccess, query.isError, query.data, setUser, clearAuth]);

  return query;
}

export function useLoginMutation() {
  const setUser = useAuthStore((s) => s.setUser);
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: LoginPayload) => authService.login(payload),
    onSuccess: (res) => {
      setUser(res.user);
      queryClient.setQueryData(['auth', 'me'], res.user);
    },
  });
}

export function useRegisterMutation() {
  const setUser = useAuthStore((s) => s.setUser);
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: RegisterPayload) => authService.register(payload),
    onSuccess: (res) => {
      setUser(res.user);
      queryClient.setQueryData(['auth', 'me'], res.user);
    },
  });
}

export function useUpdateProfileMutation() {
  const setUser = useAuthStore((s) => s.setUser);
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => authService.updateProfile(name),
    onSuccess: (profile) => {
      setUser(profile);
      queryClient.setQueryData(['auth', 'me'], profile);
    },
  });
}

export function useUserSettings() {
  const setTheme = useThemeStore((s) => s.setTheme);

  const query = useQuery({
    queryKey: ['settings'],
    queryFn: ({ signal }) => settingsService.get(signal),
    retry: false,
  });

  useEffect(() => {
    if (query.data?.theme) {
      setTheme(query.data.theme);
    }
  }, [query.data?.theme, setTheme]);

  return query;
}

export function useUpdateSettingsMutation() {
  const queryClient = useQueryClient();
  const setTheme = useThemeStore((s) => s.setTheme);

  return useMutation({
    mutationFn: (payload: UpdateSettingsPayload) => settingsService.update(payload),
    onSuccess: (updated) => {
      queryClient.setQueryData(['settings'], updated);
      setTheme(updated.theme);
    },
  });
}

