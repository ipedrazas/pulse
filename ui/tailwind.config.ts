import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        surface: {
          bg: "#0f1117",
          card: "#1a1d2e",
          border: "#2a2d3e",
        },
      },
      animation: {
        pulse_dot: "pulse_dot 2s ease-in-out infinite",
      },
      keyframes: {
        pulse_dot: {
          "0%, 100%": { opacity: "1" },
          "50%": { opacity: "0.5" },
        },
      },
    },
  },
  plugins: [],
} satisfies Config;
