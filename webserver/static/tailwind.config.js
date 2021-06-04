const colors = require('tailwindcss/colors')

module.exports = {
  mode: 'jit',
  purge: ['./src/**/*.{js,jsx,ts,tsx}', './public/index.html'],
  darkMode: 'media',
  theme: {
    colors: {
			orange: colors.orange,
			indigo: colors.indigo,
			red: colors.rose,
			green: colors.emerald,
			blue: colors.blue,
			yellow: colors.amber,
      gray: colors.gray,
      white: colors.white
    },
    extend: {},
  },
  variants: {
    extend: {},
  },
  plugins: [],
}
