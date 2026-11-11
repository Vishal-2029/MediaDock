import { API_BASE_URL, apiClient, extractData } from '@/services/apiClient';
import type { APIResponse, DownloadJob, JobStatusResponse, PageMeta } from '@/types/api';

export interface CreateDownloadPayload {
  media_id: string;
  requested_format: string;
  requested_quality?: string;
  access_token?: string;
}

export const downloadService = {
  async list(page = 1, perPage = 12, signal?: AbortSignal): Promise<{ items: DownloadJob[]; meta?: PageMeta }> {
    const res = await apiClient.get<APIResponse<DownloadJob[]>>('/downloads', {
      params: { page, per_page: perPage },
      signal,
    });
    return { items: extractData(res), meta: res.data.meta };
  },

  async create(payload: CreateDownloadPayload): Promise<DownloadJob> {
    const res = await apiClient.post<APIResponse<DownloadJob>>('/downloads', payload);
    return extractData(res);
  },

  async getById(id: string, signal?: AbortSignal): Promise<DownloadJob> {
    const res = await apiClient.get<APIResponse<DownloadJob>>(`/downloads/${id}`, { signal });
    return extractData(res);
  },

  async getStatus(id: string, signal?: AbortSignal): Promise<JobStatusResponse> {
    const res = await apiClient.get<APIResponse<JobStatusResponse>>(`/downloads/${id}/status`, { signal });
    return extractData(res);
  },

  async remove(id: string): Promise<void> {
    await apiClient.delete(`/downloads/${id}`);
  },

  getFileUrl(id: string): string {
    return `${API_BASE_URL}/downloads/${id}/file`;
  },

  async downloadFile(id: string, fallbackName = 'mediadock-file'): Promise<void> {
    const res = await apiClient.get(`/downloads/${id}/file`, { responseType: 'blob' });
    const contentType = String(res.headers['content-type'] || '');
    if (contentType.includes('application/json')) {
      const text = await (res.data as Blob).text();
      const parsed = JSON.parse(text) as APIResponse<{ download_url?: string }>;
      if (parsed.data?.download_url) {
        window.open(parsed.data.download_url, '_blank', 'noopener,noreferrer');
        return;
      }
    }

    const disposition = res.headers['content-disposition'] as string | undefined;
    const match = disposition?.match(/filename="([^"]+)"/);
    const filename = match?.[1] || fallbackName;
    const blobUrl = URL.createObjectURL(res.data as Blob);
    const link = document.createElement('a');
    link.href = blobUrl;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(blobUrl);
  },
};
