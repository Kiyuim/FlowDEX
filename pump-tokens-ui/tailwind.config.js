/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{js,jsx,ts,tsx}', './public/index.html'],
  theme: {
    extend: {
      colors: {
        // Dark trader-terminal palette
        bg: {
          DEFAULT: '#0a0b0f',
          soft: '#0f1117',
          card: '#141722',
          elev: '#1a1e2b',
          hover: '#20263a',
        },
        border: {
          DEFAULT: '#232838',
          soft: '#1b2030',
        },
        accent: {
          DEFAULT: '#00d4aa', // teal/mint
          soft: '#0b3d34',
        },
        up: '#26d07c',
        down: '#ff5c5c',
        warn: '#ffb020',
        muted: '#7c86a0',
        ink: '#e7ebf3',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'Segoe UI', 'Roboto', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 0 0 rgba(255,255,255,0.03) inset, 0 8px 24px -12px rgba(0,0,0,0.6)',
      },
      keyframes: {
        'fade-in': { from: { opacity: 0, transform: 'translateY(4px)' }, to: { opacity: 1, transform: 'none' } },
        'pulse-soft': { '0%,100%': { opacity: 1 }, '50%': { opacity: 0.5 } },
      },
      animation: {
        'fade-in': 'fade-in .2s ease-out',
        'pulse-soft': 'pulse-soft 1.5s ease-in-out infinite',
      },
    },
  },
  plugins: [],
};
