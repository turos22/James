import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

// @testing-library/react só registra a limpeza automática (unmount do DOM
// entre testes) quando enxerga um `afterEach` global. Como este projeto não
// liga `test.globals` (os arquivos de teste importam tudo explicitamente de
// "vitest"), o hook automático nunca dispara e os componentes renderizados
// em um `it` vazam pro próximo — dois botões "Negar" no DOM, por exemplo.
// Este setup registra a limpeza manualmente, sem precisar de globals.
afterEach(() => {
  cleanup();
});
