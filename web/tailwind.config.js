/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        display: ['Fraunces', 'Georgia', 'serif'],
        sans: ['Space Grotesk', 'Helvetica Neue', 'Arial', 'sans-serif'],
      },
      colors: {
        'kuberoot': {
          50: '#eefcf8',
          100: '#d7f7ee',
          500: '#0f766e',
          600: '#0b5f59',
          700: '#094a46',
          900: '#072f2c',
        }
      }
    },
  },
  plugins: [],
}
