module.exports = {
  content: ['./templates/*.html', './*.go'],
  darkMode: ['class', '.theme--dark'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Helvetica', 'ui-sans-serif', 'system-ui']
      },
      fontSize: {
        xs: '0.7rem',
        sm: '0.8rem',
        '2xl': ['1.5rem']
      },
      colors: {
        lavender: '#fdf0f5',
        strongpink: '#e32a6d',
        crimson: '#bc1150',
        garnet: '#42091e'
      },
      borderWidth: {
        '05rem': '0.5rem' // you can define custom border widths in rem units
      },
      gap: {
        '48vw': '4.8vw'
      },
      typography: ({theme}) => ({
        /* for markdown html content */
        DEFAULT: {
          css: {
            '--tw-prose-headings': theme('colors.strongpink'),
            '--tw-prose-invert-headings': theme('colors.strongpink'),
            '--tw-prose-links': theme('colors.gray[700]'),
            '--tw-prose-invert-links': theme('colors.neutral[50]')
            a: {
              'font-weight': 300,
            },
          }
        }
      })
    }
  },
  plugins: [require('@tailwindcss/typography')]
}
