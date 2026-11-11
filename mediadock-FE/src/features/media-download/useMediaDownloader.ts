import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { mediaService, type InspectMediaPayload } from '@/services/mediaService';
import { downloadService, type CreateDownloadPayload } from '@/services/downloadService';

export function useInspectMediaMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: InspectMediaPayload) => mediaService.inspect(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['media', 'list'] });
    },
  });
}

export function useCreateDownloadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateDownloadPayload) => downloadService.create(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['downloads', 'list'] });
    },
  });
}

export function useDownloadStatus(jobId: string | null) {
  return useQuery({
    queryKey: ['downloads', 'status', jobId],
    queryFn: ({ signal }) => downloadService.getStatus(jobId!, signal),
    enabled: Boolean(jobId),
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (status === 'completed' || status === 'failed' || status === 'expired') {
        return false;
      }
      return 1500;
    },
  });
}

export function useDownloadJobsList(page = 1, perPage = 12) {
  return useQuery({
    queryKey: ['downloads', 'list', page, perPage],
    queryFn: ({ signal }) => downloadService.list(page, perPage, signal),
  });
}

export function useDeleteDownloadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => downloadService.remove(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['downloads'] });
    },
  });
}

export function useMediaLibrary(page = 1, perPage = 12) {
  return useQuery({
    queryKey: ['media', 'list', page, perPage],
    queryFn: ({ signal }) => mediaService.list(page, perPage, signal),
  });
}

export function useDeleteMediaMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => mediaService.remove(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['media'] });
    },
  });
}

