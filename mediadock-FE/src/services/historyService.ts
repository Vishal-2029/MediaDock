import { apiClient, extractData } from '@/services/apiClient';
import type { APIResponse, DownloadHistoryRecord, PageMeta } from '@/types/api';

export const historyService = {
  async list(page = 1, perPage = 20, signal?: AbortSignal): Promise<{ items: DownloadHistoryRecord[]; meta?: PageMeta }> {
    const res = await apiClient.get<APIResponse<DownloadHistoryRecord[]>>('/history', {
      params: { page, per_page: perPage },
      signal,
    });
    return { items: extractData(res), meta: res.data.meta };
  },

  async getById(id: string, signal?: AbortSignal): Promise<DownloadHistoryRecord> {
    const res = await apiClient.get<APIResponse<DownloadHistoryRecord>>(`/history/${id}`, { signal });
    return extractData(res);
  },

  async remove(id: string): Promise<void> {
    await apiClient.delete(`/history/${id}`);
  },
};

