import { createApp } from "vue";
import App from "./App.vue";
import { router } from "./router";
import i18n, { detectInitialLocale } from "./i18n";
import "vue-sonner/style.css";
import "./style.css";

const app = createApp(App);
app.use(router);
app.use(i18n);

const locale = detectInitialLocale();
if (typeof document !== "undefined") {
  document.documentElement.lang = locale;
}

app.mount("#app");
