import { apiClient, extractData } from '@/services/apiClient';
import type { APIResponse, AuthResponse, UserProfile } from '@/types/api';

export interface LoginPayload {
  email: string;
  password: string;
}

export interface RegisterPayload {
  name: string;
  email: string;
  password: string;
}

export const authService = {
  async register(payload: RegisterPayload): Promise<AuthResponse> {
    const res = await apiClient.post<APIResponse<AuthResponse>>('/auth/register', payload);
    return extractData(res);
  },

  async login(payload: LoginPayload): Promise<AuthResponse> {
    const res = await apiClient.post<APIResponse<AuthResponse>>('/auth/login', payload);
    return extractData(res);
  },

  async logout(): Promise<void> {
    await apiClient.post<APIResponse<{ message: string }>>('/auth/logout', {});
  },

  async me(signal?: AbortSignal): Promise<UserProfile> {
    const res = await apiClient.get<APIResponse<UserProfile>>('/auth/me', { signal });
    return extractData(res);
  },

  async updateProfile(name: string): Promise<UserProfile> {
    const res = await apiClient.put<APIResponse<UserProfile>>('/auth/me', { name });
    return extractData(res);
  },
};

