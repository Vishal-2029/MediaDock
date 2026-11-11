import { useEffect, useState, type FormEvent } from 'react';
import { Save, SlidersHorizontal } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/common/Card';
import { Button } from '@/components/common/Button';
import { Label, Select } from '@/components/common/Input';
import { PageMeta } from '@/components/common/PageMeta';
import { useUpdateSettingsMutation, useUserSettings } from '@/features/user-settings/useAuth';
import { useToastStore } from '@/store/toastStore';
import { APIClientError } from '@/services/apiClient';
import type { ThemePreference } from '@/types/api';

export function SettingsPage() {
  const settingsQuery = useUserSettings();
  const updateSettings = useUpdateSettingsMutation();
  const pushToast = useToastStore((s) => s.push);

  const [theme, setTheme] = useState<ThemePreference>('dark');
  const [defaultFormat, setDefaultFormat] = useState('mp4');

  useEffect(() => {
    if (settingsQuery.data) {
      setTheme(settingsQuery.data.theme);
      setDefaultFormat(settingsQuery.data.default_format);
    }
  }, [settingsQuery.data]);

  const handleSavePreferences = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await updateSettings.mutateAsync({ theme, default_format: defaultFormat });
      pushToast({ title: 'Preferences saved', variant: 'success' });
    } catch (err) {
      const msg = err instanceof APIClientError ? err.message : 'Could not save preferences';
      pushToast({ title: 'Save failed', description: msg, variant: 'error' });
    }
  };

  return (
    <>
      <PageMeta title="Workspace Settings" description="Manage your MediaDock theme and output defaults." noIndex />

      <div className="mx-auto max-w-container space-y-6 px-4 py-10 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Workspace Settings</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Configure your theme preference and default output format — no login required.
          </p>
        </div>

        <Card className="max-w-xl">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <SlidersHorizontal className="h-4 w-4 text-secondary dark:text-cyanHighlight" />
              Application Preferences
            </CardTitle>
            <CardDescription>Synced with your MediaDock workspace defaults.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSavePreferences} className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="pref-theme">Theme</Label>
                <Select
                  id="pref-theme"
                  value={theme}
                  onChange={(e) => setTheme(e.target.value as ThemePreference)}
                >
                  <option value="dark">Dark (Default)</option>
                  <option value="light">Light</option>
                  <option value="system">System</option>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="pref-format">Default Output Format</Label>
                <Select
                  id="pref-format"
                  value={defaultFormat}
                  onChange={(e) => setDefaultFormat(e.target.value)}
                >
                  <option value="mp4">MP4</option>
                  <option value="mov">MOV</option>
                  <option value="mkv">MKV</option>
                  <option value="webm">WebM</option>
                  <option value="mp3">MP3</option>
                  <option value="aac">AAC</option>
                  <option value="m4a">M4A</option>
                  <option value="wav">WAV</option>
                </Select>
              </div>

              <Button type="submit" variant="secondary" isLoading={updateSettings.isPending}>
                <Save className="h-4 w-4" />
                Save Preferences
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
