import { apiClient, extractData } from '@/services/apiClient';
import type { APIResponse, ThemePreference, UserSettings } from '@/types/api';

export interface UpdateSettingsPayload {
  theme?: ThemePreference;
  default_format?: string;
}

export const settingsService = {
  async get(signal?: AbortSignal): Promise<UserSettings> {
    const res = await apiClient.get<APIResponse<UserSettings>>('/settings', { signal });
    return extractData(res);
  },

  async update(payload: UpdateSettingsPayload): Promise<UserSettings> {
    const res = await apiClient.put<APIResponse<UserSettings>>('/settings', payload);
    return extractData(res);
  },
};

