import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { historyService } from '@/services/historyService';

export function useHistoryList(page = 1, perPage = 20) {
  return useQuery({
    queryKey: ['history', 'list', page, perPage],
    queryFn: ({ signal }) => historyService.list(page, perPage, signal),
  });
}

export function useDeleteHistoryMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => historyService.remove(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['history'] });
    },
  });
}

