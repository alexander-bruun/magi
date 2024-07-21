import franken from "franken-ui/shadcn-ui/preset-quick";

/** @type {import('tailwindcss').Config} */
export default {
  presets: [franken({ theme: "zinc" })],
  content: ["./**/*.{html,js,templ}"],
  theme: {
    extend: {},
  },
  plugins: [],
};
