import { apiClient, extractData } from '@/services/apiClient';
import type { APIResponse, ConversionJob, JobStatusResponse } from '@/types/api';

export interface CreateConversionPayload {
  file: File;
  targetFormat: string;
  targetResolution?: string;
  compress?: boolean;
  extractAudio?: boolean;
  onUploadProgress?: (pct: number) => void;
}

export const conversionService = {
  async create(payload: CreateConversionPayload): Promise<ConversionJob> {
    const formData = new FormData();
    formData.append('file', payload.file);
    formData.append('target_format', payload.targetFormat);
    if (payload.targetResolution) {
      formData.append('target_resolution', payload.targetResolution);
    }
    formData.append('compress', String(Boolean(payload.compress)));
    formData.append('extract_audio', String(Boolean(payload.extractAudio)));

    const res = await apiClient.post<APIResponse<ConversionJob>>('/conversions', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000,
      onUploadProgress: (evt) => {
        if (evt.total && payload.onUploadProgress) {
          payload.onUploadProgress(Math.round((evt.loaded * 100) / evt.total));
        }
      },
    });
    return extractData(res);
  },

  async getById(id: string, signal?: AbortSignal): Promise<ConversionJob> {
    const res = await apiClient.get<APIResponse<ConversionJob>>(`/conversions/${id}`, { signal });
    return extractData(res);
  },

  async getStatus(id: string, signal?: AbortSignal): Promise<JobStatusResponse> {
    const res = await apiClient.get<APIResponse<JobStatusResponse>>(`/conversions/${id}/status`, { signal });
    return extractData(res);
  },

  async downloadFile(id: string, fallbackName = 'converted-media'): Promise<void> {
    const res = await apiClient.get(`/conversions/${id}/file`, { responseType: 'blob' });
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

