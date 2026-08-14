import { defineConfig } from "@playwright/test";

// Aponta para o jamesd de verdade, não para o vite dev: a SPA é servida
// embutida no binário (//go:embed all:dist), e é essa versão que precisa
// ser exercitada. O jamesd tem que estar de pé antes de rodar.
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:8080",
    trace: "retain-on-failure",
  },
});
