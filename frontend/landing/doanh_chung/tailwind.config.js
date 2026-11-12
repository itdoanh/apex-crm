/** Tailwind config – mirror đúng config inline trong index.html (bản local, độc lập) */
module.exports = {
  content: ['./index.html', './assets/js/main.js'],
  theme: {
    extend: {
      colors: {
        navy: { 900: '#0A192F', 800: '#0F2438', 700: '#1E293B', 600: '#1F3A5F' },
        gold: { DEFAULT: '#F5A623', light: '#FFA94D' },
        orange: { DEFAULT: '#FF6B00', dark: '#E65100' }
      },
      fontFamily: {
        sans: ['Inter', 'Be Vietnam Pro', 'system-ui', 'sans-serif'],
        heading: ['Plus Jakarta Sans', 'Be Vietnam Pro', 'sans-serif'],
        vi: ['Be Vietnam Pro', 'Inter', 'sans-serif']
      },
      animation: {
        shimmer: 'shimmer 1.5s linear infinite',
        'bounce-slow': 'bounce-slow 2.5s ease-in-out infinite'
      },
      keyframes: {
        shimmer: {
          '0%': { backgroundPosition: '200% 0' },
          '100%': { backgroundPosition: '-200% 0' }
        },
        'bounce-slow': {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-25%)' }
        }
      }
    }
  },
  safelist: [
    'group',
    'group-hover:animate-shimmer',
    'animate-shimmer',
    'animate-bounce-slow',
    { pattern: /^(hover|focus|md|lg|sm|group-hover):(bg|text|border|flex|grid|gap|p|m|w|h|px|py|pt|pb|pl|pr|mt|mb|ml|mr|items|justify|hidden|inline|block|rounded|shadow|space|tracking|leading|font|max|min|animate)-.*/ }
  ]
};
