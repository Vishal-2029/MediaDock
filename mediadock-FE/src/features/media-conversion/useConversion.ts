import { useMutation, useQuery } from '@tanstack/react-query';
import { conversionService, type CreateConversionPayload } from '@/services/conversionService';

export function useCreateConversionMutation() {
  return useMutation({
    mutationFn: (payload: CreateConversionPayload) => conversionService.create(payload),
  });
}

export function useConversionStatus(jobId: string | null) {
  return useQuery({
    queryKey: ['conversions', 'status', jobId],
    queryFn: ({ signal }) => conversionService.getStatus(jobId!, signal),
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

