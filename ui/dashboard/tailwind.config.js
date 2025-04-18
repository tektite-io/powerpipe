module.exports = {
  content: ["./src/**/*.{js,jsx,ts,tsx}", "./public/index.html"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        dashboard: "var(--color-dashboard)",
        "dashboard-panel": "var(--color-dashboard-panel)",
        foreground: "var(--color-foreground)",
        "foreground-light": "var(--color-foreground-light)",
        "foreground-lighter": "var(--color-foreground-lighter)",
        "foreground-lightest": "var(--color-foreground-lightest)",
        divide: "var(--color-divide)",
        alert: "rgb(var(--color-alert) / <alpha-value>)",
        "alert-light": "rgb(var(--color-alert) / 33.33)",
        "alert-inverse": "var(--color-alert-inverse)",
        orange: "rgb(var(--color-orange) / <alpha-value>)",
        severity: "rgb(var(--color-severity) / <alpha-value>)",
        ok: "rgb(var(--color-ok) / <alpha-value>)",
        "ok-inverse": "var(--color-ok-inverse)",
        info: "rgb(var(--color-info) / <alpha-value>)",
        "info-inverse": "var(--color-info-inverse)",
        skip: "rgb(var(--color-skip) / <alpha-value>)",
        link: "var(--color-link)",
        "table-border": "var(--color-table-border)",
        "table-divide": "var(--color-table-divide)",
        "table-head": "var(--color-table-head)",
        "slack-aubergine": "#4A154B",
        "steampipe-black": "#181717",
        "steampipe-blue-dark": "#035EDD",
        "steampipe-red": "#c7252d",
        "black-scale-1": "var(--color-black-scale-1)",
        "black-scale-2": "var(--color-black-scale-2)",
        "black-scale-3": "var(--color-black-scale-3)",
        "black-scale-4": "var(--color-black-scale-4)",
        "black-scale-5": "var(--color-black-scale-5)",
        "black-scale-6": "var(--color-black-scale-6)",
        "black-scale-7": "var(--color-black-scale-7)",
        "black-scale-8": "var(--color-black-scale-8)",
      },
      fontSize: {
        xxs: ".65rem",
      },
      maxHeight: {
        "1/2-screen": "50vh",
      },
      spacing: {
        4.5: "1.125rem",
      },
      typography: (theme) => ({
        DEFAULT: {
          css: {
            color: theme("colors.foreground"),
            a: {
              color: theme("colors.link"),
              "&:hover": {
                color: theme("colors.link"),
              },
            },
            code: { color: theme("colors.foreground") },
            "a code": { color: theme("colors.foreground") },
            h1: { color: theme("colors.foreground") },
            h2: { color: theme("colors.foreground") },
            h3: { color: theme("colors.foreground") },
            h4: { color: theme("colors.foreground") },
            h5: { color: theme("colors.foreground") },
            h6: { color: theme("colors.foreground") },
            strong: { color: theme("colors.foreground") },
            "thead tr th": {
              color: theme("colors.table-head"),
            },
            "tbody tr": { borderBottomColor: theme("colors.table-divide") },
          },
        },
      }),
    },
  },
  plugins: [require("@tailwindcss/forms"), require("@tailwindcss/typography")],
};
