/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx,js,jsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#ECFDF5',
          100: '#D1FAE5',
          200: '#A7F3D0',
          300: '#6EE7B7',
          400: '#34D399',
          500: '#10B981',
          600: '#059669',
          700: '#047857',
          800: '#065F46',
          900: '#064E3B',
        },
        income: '#10B981',
        expense: '#EF4444',
        transfer: '#6366F1',
        neutral: '#64748B',
      },
      fontFamily: {
        sans: [
          '"PingFang SC"', '"Hiragino Sans GB"', '"Microsoft YaHei"',
          'system-ui', '-apple-system', 'Segoe UI', 'Roboto',
          'sans-serif',
        ],
      },
      boxShadow: {
        card: '0 1px 2px 0 rgba(0,0,0,0.04), 0 1px 3px 0 rgba(0,0,0,0.06)',
        soft: '0 8px 30px rgba(0,0,0,0.06)',
      },
      borderRadius: {
        xl: '0.875rem',
      },
    },
  },
  plugins: [],
};
