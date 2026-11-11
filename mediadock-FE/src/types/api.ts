export interface APIErrorPayload {
  code: string;
  message: string;
  details?: Record<string, string>;
}

export interface PageMeta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: APIErrorPayload;
  meta?: PageMeta;
}

export interface UserProfile {
  id: string;
  name: string;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
}

export interface AuthResponse {
  user: UserProfile;
  tokens: TokenPair;
}

export type PlatformID = 'instagram' | 'youtube' | 'facebook' | 'generic';

export interface AuthorizationResult {
  authorized: boolean;
  authorization_method: string;
  account_identifier?: string;
  media_identifier?: string;
  verified_at?: string;
  reason?: string;
  alternative?: string;
}

export interface FormatOption {
  format: string;
  quality: string;
  container: string;
  description: string;
  audio_only?: boolean;
}

export interface MediaInspectResponse {
  media_id: string;
  source_id: string;
  provider: PlatformID;
  title: string;
  thumbnail_url?: string;
  duration: number;
  original_format: string;
  file_size: number;
  source_url: string;
  authorization: AuthorizationResult;
  available_formats: FormatOption[];
  created_at: string;
}

export interface MediaSourceRecord {
  id: string;
  user_id: string;
  provider: PlatformID;
  source_url: string;
  authorization_method: string;
  authorization_verified_at?: string;
  created_at: string;
}

export interface MediaItemRecord {
  id: string;
  user_id: string;
  source_id: string;
  source?: MediaSourceRecord;
  title: string;
  thumbnail_url?: string;
  duration: number;
  original_format: string;
  file_size: number;
  created_at: string;
}

export type JobStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'expired';

export interface DownloadJob {
  id: string;
  user_id: string;
  media_id: string;
  media?: MediaItemRecord;
  status: JobStatus;
  progress: number;
  requested_format: string;
  requested_quality: string;
  error_message?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

export interface JobStatusResponse {
  id: string;
  status: JobStatus;
  progress: number;
  error_message?: string;
  expires_at?: string;
  updated_at: string;
}

export interface DownloadHistoryRecord {
  id: string;
  user_id: string;
  job_id: string;
  file_name: string;
  downloaded_at: string;
}

export interface ConversionJob {
  id: string;
  user_id: string;
  source_filename: string;
  source_format: string;
  target_format: string;
  target_resolution?: string;
  compress: boolean;
  extract_audio: boolean;
  status: JobStatus;
  progress: number;
  file_size: number;
  duration: number;
  error_message?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

export type ThemePreference = 'light' | 'dark' | 'system';

export interface UserSettings {
  id: string;
  user_id: string;
  theme: ThemePreference;
  default_format: string;
  created_at: string;
  updated_at: string;
}

