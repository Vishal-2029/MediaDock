import { apiClient, extractData } from '@/services/apiClient';
import type { APIResponse, MediaInspectResponse, MediaItemRecord, PageMeta } from '@/types/api';

export interface InspectMediaPayload {
  url: string;
  access_token?: string;
  owner_attested?: boolean;
}

export const mediaService = {
  async inspect(payload: InspectMediaPayload, signal?: AbortSignal): Promise<MediaInspectResponse> {
    const res = await apiClient.post<APIResponse<MediaInspectResponse>>('/media/inspect', payload, { signal });
    return extractData(res);
  },

  async getById(id: string, signal?: AbortSignal): Promise<MediaInspectResponse> {
    const res = await apiClient.get<APIResponse<MediaInspectResponse>>(`/media/${id}`, { signal });
    return extractData(res);
  },

  async list(page = 1, perPage = 12, signal?: AbortSignal): Promise<{ items: MediaItemRecord[]; meta?: PageMeta }> {
    const res = await apiClient.get<APIResponse<MediaItemRecord[]>>('/media', {
      params: { page, per_page: perPage },
      signal,
    });
    return { items: extractData(res), meta: res.data.meta };
  },

  async remove(id: string): Promise<void> {
    await apiClient.delete(`/media/${id}`);
  },
};

