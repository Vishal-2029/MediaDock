import type { Config } from 'tailwindcss';

const config: Config = {
  darkMode: ['class'],
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        background: 'var(--bg-primary)',
        'background-secondary': 'var(--bg-secondary)',
        card: {
          DEFAULT: 'var(--bg-card)',
          foreground: 'var(--text-primary)',
        },
        foreground: 'var(--text-primary)',
        muted: {
          DEFAULT: 'var(--bg-secondary)',
          foreground: 'var(--text-secondary)',
        },
        border: 'var(--border-color)',
        primary: {
          DEFAULT: '#7C3AED',
          foreground: '#F8FAFC',
        },
        secondary: {
          DEFAULT: '#0891B2',
          foreground: '#F8FAFC',
        },
        cyanHighlight: '#67E8F9',
        success: '#16A34A',
        warning: '#F59E0B',
        destructive: {
          DEFAULT: '#EF4444',
          foreground: '#F8FAFC',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'sans-serif'],
      },
      maxWidth: {
        container: '1200px',
      },
      borderRadius: {
        lg: '0.75rem',
        md: '0.5rem',
        sm: '0.375rem',
      },
    },
  },
  plugins: [],
};

export default config;

